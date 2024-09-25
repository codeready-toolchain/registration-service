package proxy

import (
	gocontext "context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
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
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	commoncluster "github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	glog "github.com/labstack/gommon/log"
	errs "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/httpstream"

	"k8s.io/apiserver/pkg/util/wsstream"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultPort          = "8081"
	bearerProtocolPrefix = "base64url.bearer.authorization.k8s.io." //nolint:gosec

	proxyHealthEndpoint          = "/proxyhealth"
	authEndpoint                 = "/auth/"
	wellKnownOauthConfigEndpoint = "/.well-known/oauth-authorization-server"
	pluginsEndpoint              = "/plugins/"
)

func ssoWellKnownTarget() string {
	return fmt.Sprintf("%s/auth/realms/%s/.well-known/openid-configuration", configuration.GetRegistrationServiceConfig().Auth().SSOBaseURL(), configuration.GetRegistrationServiceConfig().Auth().SSORealm())
}

func openidAuthEndpoint() string {
	return fmt.Sprintf("/auth/realms/%s/protocol/openid-connect/auth", configuration.GetRegistrationServiceConfig().Auth().SSORealm())
}

func authorizationEndpointTarget() string {
	return fmt.Sprintf("%s%s", configuration.GetRegistrationServiceConfig().Auth().SSOBaseURL(), openidAuthEndpoint())
}

type Proxy struct {
	app            application.Application
	cl             client.Client
	tokenParser    *auth.TokenParser
	spaceLister    *handlers.SpaceLister
	metrics        *metrics.ProxyMetrics
	getMembersFunc commoncluster.GetMemberClustersFunc
}

func NewProxy(app application.Application, proxyMetrics *metrics.ProxyMetrics, getMembersFunc commoncluster.GetMemberClustersFunc) (*Proxy, error) {
	cl, err := newClusterClient()
	if err != nil {
		return nil, err
	}
	return newProxyWithClusterClient(app, cl, proxyMetrics, getMembersFunc)
}

func newProxyWithClusterClient(app application.Application, cln client.Client, proxyMetrics *metrics.ProxyMetrics, getMembersFunc commoncluster.GetMemberClustersFunc) (*Proxy, error) {
	tokenParser, err := auth.DefaultTokenParser()
	if err != nil {
		return nil, err
	}

	// init handlers
	spaceLister := handlers.NewSpaceLister(app, proxyMetrics)
	return &Proxy{
		app:            app,
		cl:             cln,
		tokenParser:    tokenParser,
		spaceLister:    spaceLister,
		metrics:        proxyMetrics,
		getMembersFunc: getMembersFunc,
	}, nil
}

func (p *Proxy) StartProxy(port string) *http.Server {
	// start server
	router := echo.New()
	router.Logger.SetLevel(glog.INFO)
	router.HTTPErrorHandler = customHTTPErrorHandler
	// middleware before routing
	router.Pre(
		p.addStartTime(),
		middleware.RemoveTrailingSlash(),
		p.stripInvalidHeaders(),
		p.addUserContext(), // get user information from token before handling request
		// log request information before routing
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(ctx echo.Context) error {
				if ctx.Request().URL.Path == proxyHealthEndpoint { // skip for health endpoint
					return next(ctx)
				}
				log.InfoEchof(ctx, "request received")
				return next(ctx)
			}
		},
		p.ensureUserIsNotBanned(),
		p.addPublicViewerContext(),
	)

	// middleware after routing
	router.Use(
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			Skipper: func(ctx echo.Context) bool {
				return ctx.Request().URL.RequestURI() == proxyHealthEndpoint // skip logging for health check, so it doesn't pollute the logs
			},
			LogMethod: true,
			LogStatus: true,
			LogURI:    true,
			LogValuesFunc: func(ctx echo.Context, _ middleware.RequestLoggerValues) error {
				log.InfoEchof(ctx, "request routed")
				return nil
			},
		}),
	)

	// routes
	wg := router.Group("/apis/toolchain.dev.openshift.com/v1alpha1/workspaces")
	// Space lister routes
	wg.GET("/:workspace", handlers.HandleSpaceGetRequest(p.spaceLister, p.getMembersFunc))
	wg.GET("", handlers.HandleSpaceListRequest(p.spaceLister))

	router.GET(proxyHealthEndpoint, p.health)
	// SSO routes. Used by web login (oc login -w).
	// Here is the expected flow for the "oc login -w" command:
	// 1. "oc login -w --server=<proxy_url>"
	// 2. oc calls <proxy_url>/.well-known/oauth-authorization-server (wellKnownOauthConfigEndpoint endpoint)
	// 3. proxy forwards it to <sso_url>/auth/realms/<sso_realm>/.well-known/openid-configuration
	// 4. oc starts an OAuth flow by opening a browser for <proxy_url>/auth/realms/<realm>/protocol/openid-connect/auth
	// 5. proxy redirects (the request is not proxied but redirected via 403 See Others response!) the request
	//    to <sso_url>/auth/realms/<realm>/protocol/openid-connect/auth
	//    Note: oc uses this hardcoded public (no secret) oauth client name: "openshift-cli-client" which has to exist in SSO to make this flow work.
	// 6. user provides the login credentials in the sso login page
	// 7. all following oc requests (<proxy_url>/auth/*) go to the proxy and forwarded to SSO as is. This is used to obtain the generated token by oc.
	router.Any(wellKnownOauthConfigEndpoint, p.oauthConfiguration)     // <- this is the step 2 in the flow above
	router.Any(fmt.Sprintf("%s*", openidAuthEndpoint()), p.openidAuth) // <- this is the step 5 in the flow above
	router.Any(fmt.Sprintf("%s*", authEndpoint), p.auth)               // <- this is the step 7.
	// The main proxy route
	router.Any("/*", p.handleRequestAndRedirect)

	// Insert the CORS preflight middleware
	handler := corsPreflightHandler(router)

	log.Info(nil, "Starting the Proxy server...")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"}, // disable HTTP/2 for now
		},
	}
	// listen concurrently to allow for graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Info(nil, fmt.Sprintf("%s - this is expected when server shutdown has been initiated", err.Error()))
			} else {
				log.Error(nil, err, err.Error())
			}
		}
	}()

	return srv
}

// unsecured returns true if the request does not require authentication
func unsecured(ctx echo.Context) bool {
	uri := ctx.Request().URL.RequestURI()
	return uri == proxyHealthEndpoint || uri == wellKnownOauthConfigEndpoint || strings.HasPrefix(uri, authEndpoint)
}

// auth handles requests to SSO. Used by web login.
func (p *Proxy) auth(ctx echo.Context) error {
	req := ctx.Request()
	targetURL, err := url.Parse(configuration.GetRegistrationServiceConfig().Auth().SSOBaseURL())
	if err != nil {
		return err
	}
	targetURL.Path = req.URL.Path
	targetURL.RawQuery = req.URL.RawQuery

	return p.handleSSORequest(targetURL)(ctx)
}

// oauthConfiguration handles requests to oauth configuration and proxies them to the corresponding SSO endpoint. Used by web login.
func (p *Proxy) oauthConfiguration(ctx echo.Context) error {
	targetURL, err := url.Parse(ssoWellKnownTarget())
	if err != nil {
		return err
	}
	return p.handleSSORequest(targetURL)(ctx)
}

// openidAuth handles requests to the openID Connect authentication endpoint. Used by web login.
func (p *Proxy) openidAuth(ctx echo.Context) error {
	targetURL, err := url.Parse(authorizationEndpointTarget())
	if err != nil {
		return err
	}
	targetURL.Path = ctx.Request().URL.Path
	targetURL.RawQuery = ctx.Request().URL.RawQuery

	// Let's redirect the browser's request to the SSO authentication page instead of proxying it
	// in order to avoid passing the user's login credentials through our proxy.
	return p.redirectTo(ctx, targetURL.String())
}

func (p *Proxy) redirectTo(ctx echo.Context, to string) error {
	log.InfoEchof(ctx, "redirecting %s to %s", ctx.Request().URL.String(), to)
	http.Redirect(ctx.Response().Writer, ctx.Request(), to, http.StatusSeeOther)
	return nil
}

// handleSSORequest handles requests to the cluster authentication server and proxy them to SSO instead. Used by web login.
func (p *Proxy) handleSSORequest(targetURL *url.URL) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		req := ctx.Request()
		director := func(req *http.Request) {
			origin := req.URL.String()
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetURL.Path
			req.URL.RawQuery = targetURL.RawQuery
			req.Host = targetURL.Host
			log.InfoEchof(ctx, "forwarding %s to %s", origin, req.URL.String())
		}
		transport := getTransport(req.Header)
		reverseProxy := &httputil.ReverseProxy{
			Director:      director,
			Transport:     transport,
			FlushInterval: -1,
		}

		// Note that ServeHttp is non-blocking and uses a go routine under the hood
		reverseProxy.ServeHTTP(ctx.Response().Writer, ctx.Request())
		return nil
	}
}

func (p *Proxy) health(ctx echo.Context) error {
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(http.StatusOK)
	_, err := io.WriteString(ctx.Response().Writer, `{"alive": true}`)
	return err
}

func (p *Proxy) processRequest(ctx echo.Context) (string, *access.ClusterAccess, error) {
	// retrieve required information from the HTTP request
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)
	proxyPluginName, workspaceName, err := getWorkspaceContext(ctx.Request())
	if err != nil {
		return "", nil, crterrors.NewBadRequest("unable to get workspace context", err.Error())
	}

	// set workspace context for logging
	ctx.Set(context.WorkspaceKey, workspaceName)

	// if the target workspace is NOT explicitly declared in the HTTP request,
	// process the request against the user's home workspace
	if workspaceName == "" {
		cluster, err := p.processHomeWorkspaceRequest(ctx, userID, username, proxyPluginName)
		if err != nil {
			return "", nil, err
		}
		return proxyPluginName, cluster, nil
	}

	// if the target workspace is explicitly declared in the HTTP request,
	// process the request against the declared workspace
	cluster, err := p.processWorkspaceRequest(ctx, userID, username, workspaceName, proxyPluginName)
	if err != nil {
		return "", nil, err
	}
	return proxyPluginName, cluster, nil
}

// processHomeWorkspaceRequest process an HTTP Request targeting the user's home workspace.
func (p *Proxy) processHomeWorkspaceRequest(ctx echo.Context, userID, username, proxyPluginName string) (*access.ClusterAccess, error) {
	// retrieves the ClusterAccess for the user and their home workspace
	cluster, err := p.app.MemberClusterService().GetClusterAccess(userID, username, "", proxyPluginName, false)
	if err != nil {
		return nil, crterrors.NewInternalError(errs.New("unable to get target cluster"), err.Error())
	}

	// list all workspaces the user has access to
	workspaces, err := handlers.ListUserWorkspaces(ctx, p.spaceLister)
	if err != nil {
		return nil, crterrors.NewInternalError(errs.New("unable to retrieve user workspaces"), err.Error())
	}

	// check whether the user has access to the home workspace
	// and whether the requestedNamespace -if any- exists in the workspace.
	requestedNamespace := namespaceFromCtx(ctx)
	if err := validateWorkspaceRequest("", requestedNamespace, workspaces...); err != nil {
		return nil, crterrors.NewForbiddenError("invalid workspace request", err.Error())
	}

	// return the cluster access
	return cluster, nil
}

// processWorkspaceRequest process an HTTP Request targeting a specific workspace.
func (p *Proxy) processWorkspaceRequest(ctx echo.Context, userID, username, workspaceName, proxyPluginName string) (*access.ClusterAccess, error) {
	// check that the user is provisioned and the space exists.
	// if the PublicViewer support is enabled, user check is skipped.
	if err := p.checkUserIsProvisionedAndSpaceExists(ctx, userID, username, workspaceName); err != nil {
		return nil, err
	}

	// retrieve the requested Workspace with SpaceBindings
	workspace, err := p.getUserWorkspaceWithBindings(ctx, workspaceName)
	if err != nil {
		return nil, err
	}

	// check whether the user has access to the workspace
	// and whether the requestedNamespace -if any- exists in the workspace.
	requestedNamespace := namespaceFromCtx(ctx)
	if err := validateWorkspaceRequest(workspaceName, requestedNamespace, *workspace); err != nil {
		return nil, crterrors.NewForbiddenError("invalid workspace request", err.Error())
	}

	// retrieve the ClusterAccess for the user and the target workspace
	return p.getClusterAccess(ctx, userID, username, proxyPluginName, workspace)
}

// checkUserIsProvisionedAndSpaceExists checks that the user is provisioned and the Space exists.
// If the PublicViewer support is enabled, User check is skipped.
func (p *Proxy) checkUserIsProvisionedAndSpaceExists(ctx echo.Context, userID, username, workspaceName string) error {
	if err := p.checkUserIsProvisioned(ctx, userID, username); err != nil {
		return crterrors.NewInternalError(errs.New("unable to get target cluster"), err.Error())
	}
	if err := p.checkSpaceExists(workspaceName); err != nil {
		return crterrors.NewInternalError(errs.New("unable to get target cluster"), err.Error())
	}
	return nil
}

// checkSpaceExists checks whether the Space exists.
func (p *Proxy) checkSpaceExists(workspaceName string) error {
	if _, err := p.app.InformerService().GetSpace(workspaceName); err != nil {
		// log the actual error but do not return it so that it doesn't reveal information about a space that may not belong to the requestor
		log.Errorf(nil, err, "requested space '%s' does not exist", workspaceName)
		return fmt.Errorf("access to workspace '%s' is forbidden", workspaceName)
	}
	return nil
}

// checkUserIsProvisioned checks whether the user is Approved, if they are not an error is returned.
// If public-viewer is enabled, user validation is skipped.
func (p *Proxy) checkUserIsProvisioned(ctx echo.Context, userID, username string) error {
	// skip if public-viewer is enabled: read-only operations on community workspaces are always permitted.
	if context.IsPublicViewerEnabled(ctx) {
		return nil
	}

	// retrieve the UserSignup for the requesting user.
	//
	// UserSignup complete status is not checked, since it might cause the proxy blocking the request
	// and returning an error when quick transitions from ready to provisioning are happening.
	userSignup, err := p.app.SignupService().GetSignupFromInformer(nil, userID, username, false)
	if err != nil {
		return err
	}

	// if the UserSignup is nil or has NOT the CompliantUsername set,
	// it means that MUR was NOT created and useraccount is NOT provisioned yet
	if userSignup == nil || userSignup.CompliantUsername == "" {
		cause := errs.New("user is not provisioned (yet)")
		log.Error(nil, cause, fmt.Sprintf("signup object: %+v", userSignup))
		return cause
	}
	return nil
}

// getClusterAccess retrieves the access to the cluster hosting the requested workspace,
// if the user has access to it.
// Access can be either direct (a SpaceBinding linking the user to the workspace exists)
// or community (a SpaceBinding linking the PublicViewer user to the workspace exists).
func (p *Proxy) getClusterAccess(ctx echo.Context, userID, username, proxyPluginName string, workspace *toolchainv1alpha1.Workspace) (*access.ClusterAccess, error) {
	// retrieve cluster access as requesting user or PublicViewer
	cluster, err := p.getClusterAccessAsUserOrPublicViewer(ctx, userID, username, proxyPluginName, workspace)
	if err != nil {
		return nil, crterrors.NewInternalError(errs.New("unable to get target cluster"), err.Error())
	}
	return cluster, nil
}

// getClusterAccessAsUserOrPublicViewer if the requesting user exists and has direct access to the workspace,
// this function returns the ClusterAccess impersonating the requesting user.
// If PublicViewer support is enabled and PublicViewer user has access to the workspace,
// this function returns the ClusterAccess impersonating the PublicViewer user.
// If requesting user does not exists and PublicViewer is disabled or does not have access to the workspace,
// this function returns an error.
func (p *Proxy) getClusterAccessAsUserOrPublicViewer(ctx echo.Context, userID, username, proxyPluginName string, workspace *toolchainv1alpha1.Workspace) (*access.ClusterAccess, error) {
	// retrieve the requesting user's UserSignup
	userSignup, err := p.app.SignupService().GetSignupFromInformer(nil, userID, username, false)
	if err != nil {
		log.Error(nil, err, fmt.Sprintf("error retrieving user signup for userID '%s' and username '%s'", userID, username))
		return nil, crterrors.NewInternalError(errs.New("unable to get user info"), "error retrieving user")
	}

	// proceed as PublicViewer if the feature is enabled and userSignup is nil
	publicViewerEnabled := context.IsPublicViewerEnabled(ctx)
	if publicViewerEnabled && !userHasDirectAccess(userSignup, workspace) {
		return p.app.MemberClusterService().GetClusterAccess(
			toolchainv1alpha1.KubesawAuthenticatedUsername,
			toolchainv1alpha1.KubesawAuthenticatedUsername,
			workspace.Name,
			proxyPluginName,
			publicViewerEnabled)
	}

	// otherwise retrieve the ClusterAccess for the cluster hosting the workspace and the given user.
	return p.app.MemberClusterService().GetClusterAccess(userID, username, workspace.Name, proxyPluginName, publicViewerEnabled)
}

// userHasDirectAccess checks if an UserSignup has access to a workspace.
// Workspace's bindings are obtained from its `status.bindings` property.
func userHasDirectAccess(signup *signup.Signup, workspace *toolchainv1alpha1.Workspace) bool {
	if signup == nil {
		return false
	}

	return userHasBinding(signup.CompliantUsername, workspace)
}

func userHasBinding(username string, workspace *toolchainv1alpha1.Workspace) bool {
	for _, b := range workspace.Status.Bindings {
		if b.MasterUserRecord == username {
			return true
		}
	}
	return false

}

// getUserWorkspaceWithBindings retrieves the workspace with the SpaceBindings if the requesting user has access to it.
// User access to the Workspace is checked by getting all spacebindings recursively,
// starting from this workspace and going up to the parent workspaces till the "root" of the workspace tree.
func (p *Proxy) getUserWorkspaceWithBindings(ctx echo.Context, workspaceName string) (*toolchainv1alpha1.Workspace, error) {
	workspace, err := handlers.GetUserWorkspaceWithBindings(ctx, p.spaceLister, workspaceName, p.getMembersFunc)
	if err != nil {
		return nil, crterrors.NewInternalError(errs.New("unable to retrieve user workspaces"), err.Error())
	}
	if workspace == nil {
		// not found
		return nil, crterrors.NewForbiddenError("invalid workspace request", fmt.Sprintf("access to workspace '%s' is forbidden", workspaceName))
	}
	// workspace was found means we can forward the request
	return workspace, nil
}

func (p *Proxy) handleRequestAndRedirect(ctx echo.Context) error {
	requestReceivedTime := ctx.Get(context.RequestReceivedTime).(time.Time)
	proxyPluginName, cluster, err := p.processRequest(ctx)
	if err != nil {
		p.metrics.RegServProxyAPIHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusNotAcceptable), metrics.MetricLabelRejected).Observe(time.Since(requestReceivedTime).Seconds())
		return err
	}
	reverseProxy := p.newReverseProxy(ctx, cluster, len(proxyPluginName) > 0)
	routeTime := time.Since(requestReceivedTime)
	p.metrics.RegServProxyAPIHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusAccepted), cluster.APIURL().Host).Observe(routeTime.Seconds())
	// Note that ServeHttp is non-blocking and uses a go routine under the hood
	reverseProxy.ServeHTTP(ctx.Response().Writer, ctx.Request())
	return nil
}

func getWorkspaceContext(req *http.Request) (string, string, error) {
	path := req.URL.Path
	proxyPluginName := ""
	// first string off any preceding proxy plugin url segment
	if strings.HasPrefix(path, pluginsEndpoint) {
		segments := strings.Split(path, "/")
		// NOTE: a split on "/plugins/" results in an array with 2 items.  One is the empty string, two is "plugins", 3 is empty string
		// behavior is not unique to "/";  "," and ",plugins," works the same way
		if len(segments) < 3 {
			return "", "", fmt.Errorf("path %q not a proxied route request", path)
		}
		if len(segments) == 3 {
			// need to distinguish between the third entry being "" vs "<plugin-name>"
			if len(strings.TrimSpace(segments[2])) == 0 {
				return "", "", fmt.Errorf("path %q not a proxied route request", path)
			}
		}
		proxyPluginName = segments[2]
		req.URL.Path = strings.TrimPrefix(path, fmt.Sprintf("/%s/%s", segments[1], segments[2]))
		path = req.URL.Path
	}
	var workspace string
	// handle specific workspace request eg. /workspaces/mycoolworkspace/api/clusterroles
	if strings.HasPrefix(path, "/workspaces/") {
		segments := strings.Split(path, "/")
		// there should be at least 4 segments eg. /workspaces/mycoolworkspace/api/clusterroles counts as 4
		if len(segments) < 4 && len(proxyPluginName) == 0 {
			return "", "", fmt.Errorf("workspace request path has too few segments '%s'; expected path format: /workspaces/<workspace_name>/api/...", path) // nolint:revive
		}
		// with proxy plugins, the route host is sufficient, and hence do not need api/...
		if len(segments) < 3 {
			return "", "", fmt.Errorf("workspace request path has too few segments '%s'; expected path format: /workspaces/<workspace_name>/<optional path>", path) // nolint:revive
		}
		if len(segments) == 3 {
			// need to distinguish between the third entry being "" vs "<plugin-name>"
			if len(strings.TrimSpace(segments[2])) == 0 {
				return "", "", fmt.Errorf("workspace request path has too few segments '%s'; expected path format: /workspaces/<workspace_name>/<optional path>", path) // nolint:revive
			}
		}
		// get the workspace segment eg. mycoolworkspace
		workspace = segments[2]
		// remove workspaces/mycoolworkspace from the request path before forwarding the request
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/workspaces/"+workspace)
	}

	return proxyPluginName, workspace, nil
}

func customHTTPErrorHandler(cause error, ctx echo.Context) {
	code := http.StatusInternalServerError
	ce := &crterrors.Error{}
	if errors.As(cause, &ce) {
		code = ce.Code
	}
	ctx.Logger().Error(cause)
	if err := ctx.String(code, cause.Error()); err != nil {
		ctx.Logger().Error(err)
	}
}

// addUserContext updates echo.Context with the User ID extracted from the Bearer token.
// To be used for storing the user ID and logging only.
func (p *Proxy) addUserContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if unsecured(ctx) { // skip only for unsecured endpoints
				return next(ctx)
			}

			token, err := p.extractUserToken(ctx.Request())
			if err != nil {
				return crterrors.NewUnauthorizedError("invalid bearer token", err.Error())
			}
			ctx.Set(context.SubKey, token.Subject)
			ctx.Set(context.UsernameKey, token.PreferredUsername)
			ctx.Set(context.EmailKey, token.Email)

			return next(ctx)
		}
	}
}

// addPublicViewerContext updates echo.Context with the configuration's PublicViewerEnabled value.
func (p *Proxy) addPublicViewerContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			publicViewerEnabled := configuration.GetRegistrationServiceConfig().PublicViewerEnabled()
			ctx.Set(context.PublicViewerEnabled, publicViewerEnabled)

			return next(ctx)
		}
	}
}

// ensureUserIsNotBanned rejects the request if the user is banned.
// This Middleware requires the context to contain the email of the user,
// so it needs to be executed after the `addUserContext` Middleware.
func (p *Proxy) ensureUserIsNotBanned() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if unsecured(ctx) { // skip only for unsecured endpoints
				return next(ctx)
			}

			email := ctx.Get(context.EmailKey).(string)
			if email == "" {
				return crterrors.NewUnauthorizedError("unauthenticated request", "invalid email in token")
			}

			// retrieve banned users
			uu, err := p.app.InformerService().ListBannedUsersByEmail(email)
			if err != nil {
				ctx.Logger().Errorf("error retrieving the list of banned users with email address %s: %v", email, err)
				return crterrors.NewInternalError(errs.New("user access could not be verified"), "could not define user access")
			}

			// if a matching Banned user is found, then user is banned
			if len(uu) > 0 {
				return crterrors.NewForbiddenError("user access is forbidden", "user access is forbidden")
			}

			// user is not banned
			return next(ctx)
		}
	}
}

func (p *Proxy) stripInvalidHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			for header := range ctx.Request().Header {
				lowercase := strings.ToLower(header)
				if strings.HasPrefix(lowercase, "impersonate-") {
					log.Info(nil, fmt.Sprintf("Removing invalid header %s from context '%+v'", header, ctx))
					ctx.Request().Header.Del(header)
				}
			}
			return next(ctx)
		}
	}
}

func (p *Proxy) addStartTime() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if ctx.Request().URL.Path == proxyHealthEndpoint { // skip only for health endpoint
				return next(ctx)
			}
			ctx.Set(context.RequestReceivedTime, time.Now())
			return next(ctx)
		}
	}
}

func (p *Proxy) extractUserToken(req *http.Request) (*auth.TokenClaims, error) {
	userToken := ""
	var err error
	if wsstream.IsWebSocketRequest(req) {
		userToken, err = extractTokenFromWebsocketRequest(req)
		if err != nil {
			return nil, err
		}
	} else {
		userToken, err = extractUserToken(req)
		if err != nil {
			return nil, err
		}
	}

	token, err := p.tokenParser.FromString(userToken)
	if err != nil {
		return nil, crterrors.NewUnauthorizedError("unable to extract userID from token", err.Error())
	}
	return token, nil
}

func extractUserToken(req *http.Request) (string, error) {
	a := req.Header.Get("Authorization")
	token := strings.Split(a, "Bearer ")
	if len(token) < 2 {
		return "", crterrors.NewUnauthorizedError("no token found", "a Bearer token is expected")
	}
	return token[1], nil
}

func (p *Proxy) newReverseProxy(ctx echo.Context, target *access.ClusterAccess, isPlugin bool) *httputil.ReverseProxy {
	req := ctx.Request()
	targetQuery := target.APIURL().RawQuery
	username, _ := ctx.Get(context.UsernameKey).(string)
	// set username in context for logging purposes
	ctx.Set(context.ImpersonateUser, target.Username())

	director := func(req *http.Request) {
		origin := req.URL.String()
		req.URL.Scheme = target.APIURL().Scheme
		req.URL.Host = target.APIURL().Host
		req.URL.Path = singleJoiningSlash(target.APIURL().Path, req.URL.Path)
		req.Header.Set("X-SSO-User", username)

		if isPlugin {
			// for non k8s clients testing, like vanilla http clients accessing plugin proxy flows, testing has proven that the request
			// host needs to be updated in addition to the URL in order to have the reverse proxy contact the openshift
			// route on the member cluster
			req.Host = target.APIURL().Host
		}
		log.InfoEchof(ctx, "forwarding %s to %s", origin, req.URL.String())
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
	transport := getTransport(req.Header)
	m := &responseModifier{req.Header.Get("Origin")}
	return &httputil.ReverseProxy{
		Director:       director,
		Transport:      transport,
		FlushInterval:  -1,
		ModifyResponse: m.addCorsToResponse,
	}
}

// TODO: use transport from the cached ToolchainCluster instance
func noTimeoutDefaultTransport() *http.Transport {
	transport := http.DefaultTransport.(interface {
		Clone() *http.Transport
	}).Clone()
	transport.DialContext = noTimeoutDialerProxy
	return transport
}

var noTimeoutDialerProxy = func(ctx gocontext.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   0,
		KeepAlive: 0,
	}
	return dialer.DialContext(ctx, network, addr)
}

func getTransport(reqHeader http.Header) *http.Transport {
	// TODO: use transport from the cached ToolchainCluster instance
	transport := noTimeoutDefaultTransport()

	if !configuration.GetRegistrationServiceConfig().IsProdEnvironment() {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // nolint:gosec
		}
	}

	// for exec and rsh command we cannot use h2 because it doesn't support "Upgrade: SPDY/3.1" header https://github.com/kubernetes/kubernetes/issues/7452
	if strings.HasPrefix(strings.ToLower(reqHeader.Get(httpstream.HeaderUpgrade)), "spdy/") {
		// thus, we need to switch to http/1.1
		transport.ForceAttemptHTTP2 = false
		transport.TLSClientConfig = &tls.Config{ // nolint:gosec
			NextProtos: []string{"http/1.1"},
		}
	}

	return transport
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

// validateWorkspaceRequest checks whether the requested workspace is in the list of workspaces the user has visibility on (retrieved via the spaceLister).
// If `requestedWorkspace` is zero, this function looks for the home workspace (the one with `status.Type` set to `home`).
// If `requestedNamespace` is NOT zero, this function checks if the namespace exists in the workspace.
func validateWorkspaceRequest(requestedWorkspace, requestedNamespace string, workspaces ...toolchainv1alpha1.Workspace) error {
	// check workspace access
	isHomeWSRequested := requestedWorkspace == ""

	allowedWorkspace := -1
	for i, w := range workspaces {
		if w.Name == requestedWorkspace || (isHomeWSRequested && w.Status.Type == "home") {
			allowedWorkspace = i
			break
		}
	}
	if allowedWorkspace == -1 {
		return fmt.Errorf("access to workspace '%s' is forbidden", requestedWorkspace)
	}

	// check namespace access
	if requestedNamespace != "" {
		allowedNamespace := false
		namespaces := workspaces[allowedWorkspace].Status.Namespaces
		for _, ns := range namespaces {
			if ns.Name == requestedNamespace {
				allowedNamespace = true
				break
			}
		}
		if !allowedNamespace {
			return fmt.Errorf("access to namespace '%s' in workspace '%s' is forbidden", requestedNamespace, workspaces[allowedWorkspace].Name)
		}
	}
	return nil
}

func namespaceFromCtx(ctx echo.Context) string {
	path := ctx.Request().URL.Path
	if strings.Index(path, "/namespaces/") > 0 {
		segments := strings.Split(path, "/")
		for i, segment := range segments {
			if segment == "namespaces" && i+1 < len(segments) {
				return segments[i+1]
			}
		}
	}
	return ""
}
