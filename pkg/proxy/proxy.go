package proxy

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	signupsvc "github.com/codeready-toolchain/registration-service/pkg/signup/service"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"

	"github.com/gin-gonic/gin"
	errs "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/util/wsstream"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerlog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ProxyPort            = "8081"
	bearerProtocolPrefix = "base64url.bearer.authorization.k8s.io." //nolint:gosec
)

type Proxy struct {
	app         application.Application
	cl          client.Client
	informer    *Informer
	tokenParser *auth.TokenParser
}

func NewProxy(app application.Application) (*Proxy, error) {
	cl, err := newClusterClient()
	if err != nil {
		return nil, err
	}
	return newProxyWithClusterClient(app, cl)
}

func newProxyWithClusterClient(app application.Application, cln client.Client) (*Proxy, error) {
	// Initiate toolchain cluster cache service
	cacheLog := controllerlog.Log.WithName("registration-service")
	cluster.NewToolchainClusterService(cln, cacheLog, configuration.Namespace(), 5*time.Second)

	tokenParser, err := auth.DefaultTokenParser()
	if err != nil {
		return nil, err
	}
	return &Proxy{
		app:         app,
		cl:          cln,
		tokenParser: tokenParser,
	}, nil
}

func (p *Proxy) StartProxy(cfg *rest.Config) *http.Server {
	// start server
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequestAndRedirect)
	mux.HandleFunc("/proxyhealth", p.health)

	// Insert the CORS preflight middleware
	handler := corsPreflightHandler(mux)

	// listen concurrently to allow for graceful shutdown
	log.Info(nil, "Starting the Proxy server...")
	srv := &http.Server{Addr: ":" + ProxyPort, Handler: handler}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error(nil, err, err.Error())
		}
	}()

	// start the informer to start watching UserSignups to invalidate cache
	informer, stopper, err := StartInformer(cfg)
	if err != nil {
		log.Error(nil, err, err.Error())
	}
	p.informer = informer

	// stop the cache invalidator on proxy server shutdown
	srv.RegisterOnShutdown(func() {
		stopper <- struct{}{}
	})

	return srv
}

func (p *Proxy) health(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	_, err := io.WriteString(res, `{"alive": true}`)
	if err != nil {
		log.Error(nil, err, "failed to write health response")
	}
}

func (p *Proxy) handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	ctx, err := p.createContext(req)
	if err != nil {
		log.Error(nil, err, "invalid bearer token")
		responseWithError(res, crterrors.NewUnauthorizedError("invalid bearer token", err.Error()))
		return
	}
	cluster, err := p.getClusterAccess(ctx)
	if err != nil {
		log.Error(ctx, err, "unable to get target cluster")
		responseWithError(res, crterrors.NewInternalError(errs.New("unable to get target cluster"), err.Error()))
		return
	}

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	p.newReverseProxy(ctx, req, cluster).ServeHTTP(res, req)
}

func responseWithError(res http.ResponseWriter, err *crterrors.Error) {
	http.Error(res, err.Error(), err.Code)
}

// createContext creates a new gin.Context with the User ID extracted from the Bearer token.
// To be used for storing the user ID and logging only.
func (p *Proxy) createContext(req *http.Request) (*gin.Context, error) {
	userID, username, err := p.extractUserID(req)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]interface{})
	keys[context.SubKey] = userID
	keys[context.UsernameKey] = username
	return &gin.Context{
		Keys: keys,
	}, nil
}

func (p *Proxy) getClusterAccess(ctx *gin.Context) (*access.ClusterAccess, error) {
	userID := ctx.GetString(context.SubKey)
	username := ctx.GetString(context.UsernameKey)
	log.Infof(nil, "Getting cluster access for user with username '%s' and ID '%s'", username, userID)

	signup, err := p.GetUserSignup(userID, username)
	if err != nil {
		return nil, err
	}

	if signup == nil || signup.Status.CompliantUsername == "" {
		return nil, errs.New("user is not approved (yet)")
	}

	log.Infof(nil, "Getting MasterUserRecord '%s'", signup.Status.CompliantUsername)
	mur, err := p.informer.GetMasterUserRecord(signup.Status.CompliantUsername)
	if err != nil {
		return nil, err
	}
	log.Infof(nil, "Found MasterUserRecord '%s'", fmt.Sprintf("%+v", mur))

	murCondition, _ := condition.FindConditionByType(mur.Status.Conditions, toolchainv1alpha1.ConditionReady)
	ready, err := strconv.ParseBool(string(murCondition.Status))
	if err != nil {
		return nil, errs.Wrapf(err, "unable to parse readiness status as bool: %s", murCondition.Status)
	}

	if !ready {
		return nil, errs.New("user is not provisioned (yet)")
	}

	log.Info(nil, "Looking up target member cluster")
	// Get the target member
	members := cluster.GetMemberClusters()
	if len(members) == 0 {
		return nil, errs.New("no member clusters found")
	}
	if len(mur.Status.UserAccounts) == 0 {
		return nil, errs.New("no useraccounts found")
	}
	for _, member := range members {
		if member.Name == mur.Status.UserAccounts[0].Cluster.Name {
			apiURL, err := url.Parse(member.APIEndpoint)
			if err != nil {
				return nil, err
			}
			// requests use impersonation so are made with member ToolchainCluster token, not user tokens
			token := member.RestConfig.BearerToken
			log.Infof(nil, "Returning ClusterAccess with API URL '%s', username '%s'", apiURL.Path, signup.Status.CompliantUsername)
			return access.NewClusterAccess(*apiURL, p.cl, token, signup.Status.CompliantUsername), nil
		}
	}

	return nil, errs.New("no member cluster found for the user")
}

// GetUserSignup is used to return the actual UserSignup resource instance, rather than the Signup DTO
func (p *Proxy) GetUserSignup(userID, username string) (*toolchainv1alpha1.UserSignup, error) {
	// Retrieve UserSignup resource from the host cluster
	userSignup, err := p.informer.GetUserSignup(signupsvc.EncodeUserIdentifier(username))
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Capture any error here in a separate var, as we need to preserve the original
			userSignup, err2 := p.informer.GetUserSignup(signupsvc.EncodeUserIdentifier(userID))
			if err2 != nil {
				if apierrors.IsNotFound(err2) {
					return nil, err
				}
				return nil, err2
			}
			return userSignup, nil
		}
		return nil, err
	}

	return userSignup, nil
}

func (p *Proxy) extractUserID(req *http.Request) (string, string, error) {
	userToken := ""
	var err error
	if wsstream.IsWebSocketRequest(req) {
		userToken, err = extractTokenFromWebsocketRequest(req)
		if err != nil {
			return "", "", err
		}
	} else {
		userToken, err = extractUserToken(req)
		if err != nil {
			return "", "", err
		}
	}

	token, err := p.tokenParser.FromString(userToken)
	if err != nil {
		return "", "", crterrors.NewUnauthorizedError("unable to extract userID from token", err.Error())
	}
	return token.Subject, token.PreferredUsername, nil
}

func extractUserToken(req *http.Request) (string, error) {
	a := req.Header.Get("Authorization")
	token := strings.Split(a, "Bearer ")
	if len(token) < 2 {
		return "", crterrors.NewUnauthorizedError("no token found", "a Bearer token is expected")
	}
	return token[1], nil
}

func (p *Proxy) newReverseProxy(ctx *gin.Context, req *http.Request, target *access.ClusterAccess) *httputil.ReverseProxy {
	targetQuery := target.APIURL().RawQuery
	director := func(req *http.Request) {
		origin := req.URL.String()
		req.URL.Scheme = target.APIURL().Scheme
		req.URL.Host = target.APIURL().Host
		req.URL.Path = singleJoiningSlash(target.APIURL().Path, req.URL.Path)
		log.Info(ctx, fmt.Sprintf("forwarding %s to %s", origin, req.URL.String()))
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
		// Replace token
		if wsstream.IsWebSocketRequest(req) {
			replaceTokenInWebsocketRequest(req, target.ImpersonatorToken())
		} else {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", target.ImpersonatorToken()))
		}

		// Set impersonation header
		req.Header.Set("Impersonate-User", target.Username())
	}
	transport := http.DefaultTransport
	if !configuration.GetRegistrationServiceConfig().IsProdEnvironment() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // nolint:gosec
			},
		}
	}
	m := &responseModifier{req.Header.Get("Origin")}
	return &httputil.ReverseProxy{
		Director:       director,
		Transport:      transport,
		FlushInterval:  -1,
		ModifyResponse: m.addCorsToResponse,
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func newClusterClient() (client.Client, error) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := toolchainv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	cl, err := client.New(k8sConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, errs.Wrap(err, "cannot create ToolchainCluster client")
	}
	return cl, nil
}

var ph = textproto.CanonicalMIMEHeaderKey("Sec-WebSocket-Protocol")

func extractTokenFromWebsocketRequest(req *http.Request) (string, error) {
	token := ""
	sawTokenProtocol := false
	for _, protocolHeader := range req.Header[ph] {
		for _, protocol := range strings.Split(protocolHeader, ",") {
			protocol = strings.TrimSpace(protocol)
			if !strings.HasPrefix(protocol, bearerProtocolPrefix) {
				continue
			}

			if sawTokenProtocol {
				return "", errs.New("multiple base64.bearer.authorization tokens specified")
			}
			sawTokenProtocol = true

			encodedToken := strings.TrimPrefix(protocol, bearerProtocolPrefix)
			decodedToken, err := base64.RawURLEncoding.DecodeString(encodedToken)
			if err != nil {
				return "", errs.Wrap(err, "invalid base64.bearer.authorization token encoding")
			}
			if !utf8.Valid(decodedToken) {
				return "", errs.New("invalid base64.bearer.authorization token: contains non UTF-8-encoded runes")
			}
			token = string(decodedToken)
		}
	}

	if len(token) == 0 {
		return "", errs.New("no base64.bearer.authorization token found")
	}

	return token, nil
}

func replaceTokenInWebsocketRequest(req *http.Request, newToken string) {
	var protocols []string
	encodedToken := base64.RawURLEncoding.EncodeToString([]byte(newToken))
	for _, protocolHeader := range req.Header[ph] {
		for _, protocol := range strings.Split(protocolHeader, ",") {
			protocol = strings.TrimSpace(protocol)
			if !strings.HasPrefix(protocol, bearerProtocolPrefix) {
				protocols = append(protocols, protocol)
				continue
			}
			// Replace the token
			protocols = append(protocols, bearerProtocolPrefix+encodedToken)
		}
	}
	req.Header.Set(ph, strings.Join(protocols, ","))
}
