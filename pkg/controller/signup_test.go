package controller_test

import (
	"bytes"
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/verification/service"
	verification_service "github.com/codeready-toolchain/registration-service/pkg/verification/service"
	"github.com/codeready-toolchain/registration-service/test"
	testutil "github.com/codeready-toolchain/registration-service/test/util"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"
	testsocialevent "github.com/codeready-toolchain/toolchain-common/pkg/test/socialevent"
	testusersignup "github.com/codeready-toolchain/toolchain-common/pkg/test/usersignup"
	"github.com/codeready-toolchain/toolchain-common/pkg/usersignup"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TestSignupSuite struct {
	test.UnitTestSuite
	httpClient *http.Client
}

func TestRunSignupSuite(t *testing.T) {
	suite.Run(t, &TestSignupSuite{test.UnitTestSuite{}, nil})
}

func httpClientFactoryOption() func(serviceFactory *factory.ServiceFactory) {
	httpClient := &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(httpClient)

	serviceOption := func(svc *verification_service.ServiceImpl) {
		svc.HTTPClient = httpClient
	}

	opt := func(serviceFactory *factory.ServiceFactory) {
		serviceFactory.WithVerificationServiceOption(serviceOption)
	}

	return opt
}

func (s *TestSignupSuite) TestSignupPostHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to true")

	s.Run("signup created", func() {
		// given
		fakeClient, application := testutil.PrepareInClusterApp(s.T())
		signupCtrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(signupCtrl.PostHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Put userID to the context
		ob, err := uuid.NewV4()
		require.NoError(s.T(), err)
		expectedUserID := ob.String()
		ctx.Set(context.SubKey, expectedUserID)
		ctx.Set(context.UsernameKey, "bill@kubesaw")
		ctx.Set(context.EmailKey, expectedUserID+"@test.com")

		// when
		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusAccepted, rr.Code)
		userSignup := &crtapi.UserSignup{}
		require.NoError(s.T(), fakeClient.Get(ctx,
			commontest.NamespacedName(commontest.HostOperatorNs, usersignup.EncodeUserIdentifier("bill@kubesaw")), userSignup))
		assert.Equal(s.T(), expectedUserID, userSignup.Spec.IdentityClaims.Sub)
		assert.Equal(s.T(), "bill@kubesaw", userSignup.Spec.IdentityClaims.PreferredUsername)
		assert.Equal(s.T(), expectedUserID+"@test.com", userSignup.Spec.IdentityClaims.Email)
	})

	s.Run("signup error", func() {
		// given
		fakeClient, application := testutil.PrepareInClusterApp(s.T())
		signupCtrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(signupCtrl.PostHandler)
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		fakeClient.MockCreate = func(_ gocontext.Context, _ client.Object, _ ...client.CreateOption) error {
			return errors.New("blah")
		}

		// when
		handler(ctx)

		// Check the error is what we expect.
		test.AssertError(s.T(), rr, http.StatusInternalServerError, "blah", "error creating UserSignup resource")
	})

	s.Run("signup forbidden error", func() {
		// given
		svc := &FakeSignupService{}
		_, application := testutil.PrepareInClusterAppWithOption(s.T(), func(serviceFactory *factory.ServiceFactory) {
			serviceFactory.WithSignupService(svc)
		})

		signupCtrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(signupCtrl.PostHandler)
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// when
		svc.MockSignup = func(_ *gin.Context) (*crtapi.UserSignup, error) {
			return nil, apierrors.NewForbidden(schema.GroupResource{}, "", errors.New("forbidden test error"))
		}

		handler(ctx)

		// Check the error is what we expect.
		test.AssertError(s.T(), rr, http.StatusForbidden, "forbidden: forbidden test error", "error creating UserSignup resource")
	})
}

func (s *TestSignupSuite) TestSignupGetHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	// Create a mock SignupService
	svc := &FakeSignupService{}
	_, application := testutil.PrepareInClusterAppWithOption(s.T(), func(serviceFactory *factory.ServiceFactory) {
		serviceFactory.WithSignupService(svc)
	})

	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	username := ob.String()

	// Create Signup controller instance.
	ctrl := controller.NewSignup(application)
	handler := gin.HandlerFunc(ctrl.GetHandler)

	s.Run("signups found", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.UsernameKey, username)

		targetCluster, err := uuid.NewV4()
		require.NoError(s.T(), err)

		expected := &signup.Signup{
			ConsoleURL:      "https://console." + targetCluster.String(),
			CheDashboardURL: "http://che-toolchain-che.member-123.com",
			APIEndpoint:     "http://api.devcluster.openshift.com",
			Username:        "jsmith",
			Status: signup.Status{
				Reason: "Provisioning",
			},
		}
		svc.MockGetSignup = func(_ *gin.Context, _, name string, _ bool) (*signup.Signup, error) {
			if name == username {
				return expected, nil
			}
			return nil, nil
		}

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &signup.Signup{}
		err = json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), expected, data)
	})

	s.Run("signups not found", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.UsernameKey, username)

		svc.MockGetSignup = func(_ *gin.Context, _, _ string, _ bool) (*signup.Signup, error) {
			return nil, nil
		}

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusNotFound, rr.Code, "handler returned wrong status code")
	})

	s.Run("signups service error", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.UsernameKey, username)

		svc.MockGetSignup = func(_ *gin.Context, _, _ string, _ bool) (*signup.Signup, error) {
			return nil, errors.New("oopsie woopsie")
		}

		handler(ctx)

		// Check the error is what we expect.
		test.AssertError(s.T(), rr, http.StatusInternalServerError, "oopsie woopsie", "error getting UserSignup resource")
	})
}

func (s *TestSignupSuite) TestInitVerificationHandler() {
	// call override config to ensure the factory option takes effect
	s.OverrideApplicationDefault()

	// Create UserSignup
	userSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("johnny@kubesaw"),
		testusersignup.WithAnnotation(crtapi.UserSignupVerificationCounterAnnotationKey, "0"),
		testusersignup.WithAnnotation(crtapi.UserSignupVerificationCodeAnnotationKey, ""),
		testusersignup.VerificationRequiredAgo(time.Second))
	fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup)
	defer gock.Off()

	// Create Signup controller instance.
	ctrl := controller.NewSignup(application)
	handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

	assertInitVerificationSuccess := func(phoneNumber, expectedHash string, expectedCounter int) {
		gock.New("https://api.twilio.com").
			Reply(http.StatusNoContent).
			BodyString("")

		data := []byte(fmt.Sprintf(`{"phone_number": "%s", "country_code": "1"}`, phoneNumber))
		rr := initPhoneVerification(s.T(), handler, gin.Param{}, data, "johnny@kubesaw", http.MethodPut, "/api/v1/signup/verification")
		require.Equal(s.T(), http.StatusNoContent, rr.Code)

		updatedUserSignup := &crtapi.UserSignup{}
		err := fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), updatedUserSignup)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), updatedUserSignup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.NotEmpty(s.T(), updatedUserSignup.Annotations[crtapi.UserSignupVerificationInitTimestampAnnotationKey])
		require.NotEmpty(s.T(), updatedUserSignup.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
		require.Equal(s.T(), strconv.Itoa(expectedCounter), updatedUserSignup.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
		require.Equal(s.T(), expectedHash, updatedUserSignup.Labels[crtapi.UserSignupUserPhoneHashLabelKey])
	}

	s.Run("init verification success", func() {
		assertInitVerificationSuccess("2268213044", "fd276563a8232d16620da8ec85d0575f", 1)
	})

	s.Run("init verification success phone number with parenthesis and spaces", func() {
		assertInitVerificationSuccess("(226) 821 3045", "9691252ac0ea2cb55295ac9b98df1c51", 2)
	})

	s.Run("init verification success phone number with dashes", func() {
		assertInitVerificationSuccess("226-821-3044", "fd276563a8232d16620da8ec85d0575f", 3)
	})
	s.Run("init verification success phone number with spaces", func() {
		assertInitVerificationSuccess("2 2 6 8 2 1 3 0 4 7", "ce3e697125f35efb76357ed8e3b768b7", 4)
	})
	s.Run("init verification fails with invalid country code", func() {
		gock.New("https://api.twilio.com").
			Reply(http.StatusNoContent).
			BodyString("")

		data := []byte(`{"phone_number": "2268213044", "country_code": "(1)"}`)
		rr := initPhoneVerification(s.T(), handler, gin.Param{}, data, "johnny@kubesaw", http.MethodPut, "/api/v1/signup/verification")
		require.Equal(s.T(), http.StatusBadRequest, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Bad Request", bodyParams["status"])
		require.InDelta(s.T(), float64(400), bodyParams["code"], 0.01)
		require.Equal(s.T(), "strconv.Atoi: parsing \"(1)\": invalid syntax", bodyParams["message"])
		require.Equal(s.T(), "invalid country_code", bodyParams["details"])
	})
	s.Run("init verification request body could not be read", func() {
		data := []byte(`{"test_number": "2268213044", "test_code": "1"}`)
		rr := initPhoneVerification(s.T(), handler, gin.Param{}, data, "johnny@kubesaw", http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		messageLines := strings.Split(bodyParams["message"].(string), "\n")
		require.Equal(s.T(), "Bad Request", bodyParams["status"])
		require.InDelta(s.T(), float64(400), bodyParams["code"], 0.01)
		require.Contains(s.T(), messageLines, "Key: 'Phone.CountryCode' Error:Field validation for 'CountryCode' failed on the 'required' tag")
		require.Contains(s.T(), messageLines, "Key: 'Phone.PhoneNumber' Error:Field validation for 'PhoneNumber' failed on the 'required' tag")
		require.Equal(s.T(), "error reading request body", bodyParams["details"])
	})

	s.Run("init verification daily limit exceeded", func() {
		cfg := configuration.GetRegistrationServiceConfig()
		originalValue := cfg.Verification().DailyLimit()
		s.SetConfig(testconfig.RegistrationService().Verification().DailyLimit(0))
		defer s.SetConfig(testconfig.RegistrationService().Verification().DailyLimit(originalValue))

		data := []byte(`{"phone_number": "2268213044", "country_code": "1"}`)
		rr := initPhoneVerification(s.T(), handler, gin.Param{}, data, "johnny@kubesaw", http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusForbidden, rr.Code, "handler returned wrong status code")
	})

	s.Run("init verification handler fails when verification not required", func() {
		// Create UserSignup
		userSignup := testusersignup.NewUserSignup(testusersignup.WithEncodedName("johnny@kubesaw"))

		_, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

		data := []byte(`{"phone_number": "2268213044", "country_code": "1"}`)
		rr := initPhoneVerification(s.T(), handler, gin.Param{}, data, "johnny@kubesaw", http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Bad Request", bodyParams["status"])
		require.InDelta(s.T(), float64(400), bodyParams["code"], 0.01)
		require.Equal(s.T(), "forbidden request: verification code will not be sent", bodyParams["message"])
		require.Equal(s.T(), "forbidden request", bodyParams["details"])
	})

	s.Run("init verification handler fails when invalid phone number provided", func() {
		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		data := []byte(`{"phone_number": "!226%213044", "country_code": "1"}`)
		rr := initPhoneVerification(s.T(), handler, gin.Param{}, data, "johnny@kubesaw", http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	})
}

func (s *TestSignupSuite) TestVerifyPhoneCodeHandler() {
	// Create UserSignup
	userSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("johnny@kubesaw"),
		testusersignup.WithAnnotation(crtapi.UserVerificationAttemptsAnnotationKey, "0"),
		testusersignup.WithAnnotation(crtapi.UserSignupVerificationCodeAnnotationKey, "999888"),
		testusersignup.WithAnnotation(crtapi.UserVerificationExpiryAnnotationKey, time.Now().Add(10*time.Second).Format(service.TimestampLayout)))

	s.Run("verification successful", func() {
		// Create Signup controller instance.
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.VerifyPhoneCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "999888",
		}
		rr := initPhoneVerification(s.T(), handler, param, nil, "johnny@kubesaw", http.MethodGet, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		updatedUserSignup := &crtapi.UserSignup{}
		err := fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), updatedUserSignup)
		require.NoError(s.T(), err)

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		require.False(s.T(), states.VerificationRequired(updatedUserSignup))
		require.Empty(s.T(), updatedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		require.Empty(s.T(), updatedUserSignup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Empty(s.T(), updatedUserSignup.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
	})

	s.Run("getsignup returns error", func() {
		// Simulate returning an error
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			return errors.New("no user")
		}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.VerifyPhoneCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "111233",
		}
		rr := initPhoneVerification(s.T(), handler, param, nil, "johnny@kubesaw", http.MethodGet, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Internal Server Error", bodyParams["status"])
		require.InDelta(s.T(), float64(500), bodyParams["code"], 0.01)
		require.Equal(s.T(), "no user: error retrieving usersignup: ", bodyParams["message"])
		require.Equal(s.T(), "error while verifying phone code", bodyParams["details"])
	})

	s.Run("getsignup returns nil", func() {
		_, application := testutil.PrepareInClusterApp(s.T())

		// Create Signup controller instance and handle the verification request
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.VerifyPhoneCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "111233",
		}
		rr := initPhoneVerification(s.T(), handler, param, nil, "jsmith@kubesaw", http.MethodGet, "/api/v1/signup/verification/111233")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusNotFound, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Not Found", bodyParams["status"])
		require.InDelta(s.T(), float64(404), bodyParams["code"], 0.01)
		// the fdebf2d6-jsmithkubesaw is an encoded version of the jsmith@kubesaw username (removed @ and prefixed with crc32 hash of the original value)
		require.Equal(s.T(), "usersignups.toolchain.dev.openshift.com \"fdebf2d6-jsmithkubesaw\" not found: user not found", bodyParams["message"])
		require.Equal(s.T(), "error while verifying phone code", bodyParams["details"])
	})

	s.Run("update usersignup returns error", func() {
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)
		fakeClient.MockUpdate = func(_ gocontext.Context, _ client.Object, _ ...client.UpdateOption) error {
			return apierrors.NewServiceUnavailable("service unavailable")
		}
		// Create Signup controller instance.
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.VerifyPhoneCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "555555",
		}
		rr := initPhoneVerification(s.T(), handler, param, nil, "johnny@kubesaw", http.MethodGet,
			"/api/v1/signup/verification/555555")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Internal Server Error", bodyParams["status"])
		require.InDelta(s.T(), float64(500), bodyParams["code"], 0.01)
		require.Equal(s.T(), "there was an error while updating your account - please wait a moment before "+
			"trying again. If this error persists, please contact the Developer Sandbox team at devsandbox@redhat.com for "+
			"assistance: error while verifying phone code", bodyParams["message"])
		require.Equal(s.T(), "unexpected error while verifying phone code", bodyParams["details"])
	})

	s.Run("verifycode returns status error", func() {

		userSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey] = "9999"
		userSignup.Annotations[crtapi.UserVerificationExpiryAnnotationKey] = time.Now().Add(10 * time.Second).Format(service.TimestampLayout)
		userSignup.Annotations[crtapi.UserSignupVerificationTimestampAnnotationKey] = time.Now().Format(service.TimestampLayout)

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.VerifyPhoneCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "333333",
		}
		rr := initPhoneVerification(s.T(), handler, param, nil, "johnny@kubesaw", http.MethodGet, "/api/v1/signup/verification/333333")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusTooManyRequests, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Too Many Requests", bodyParams["status"])
		require.InDelta(s.T(), float64(429), bodyParams["code"], 0.01)
		require.Equal(s.T(), "too many verification attempts", bodyParams["message"])
		require.Equal(s.T(), "error while verifying phone code", bodyParams["details"])
	})

	s.Run("no code provided", func() {
		// Create Signup controller instance.
		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.VerifyPhoneCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "",
		}
		rr := initPhoneVerification(s.T(), handler, param, nil, "", http.MethodGet, "/api/v1/signup/verification/")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusBadRequest, rr.Code)
	})
}

func initPhoneVerification(t *testing.T, handler gin.HandlerFunc, params gin.Param, data []byte, username, httpMethod, url string) *httptest.ResponseRecorder {
	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	req, err := http.NewRequest(httpMethod, url, bytes.NewBuffer(data))
	require.NoError(t, err)
	ctx.Request = req
	ctx.Set(context.UsernameKey, username)

	ctx.Params = append(ctx.Params, params)
	handler(ctx)

	return rr
}

func (s *TestSignupSuite) TestVerifyActivationCodeHandler() {

	s.Run("verification successful", func() {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second)) // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event")
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)
		ctrl := controller.NewSignup(application)
		handler := gin.HandlerFunc(ctrl.VerifyActivationCodeHandler)

		// when
		rr := initActivationCodeVerification(s.T(), handler, userSignup.Name, event.Name)

		// then
		require.Equal(s.T(), http.StatusOK, rr.Code)
		updatedUserSignup := &crtapi.UserSignup{}
		err := fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), updatedUserSignup)
		require.NoError(s.T(), err)
		require.False(s.T(), states.VerificationRequired(updatedUserSignup))
		require.Empty(s.T(), updatedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		require.Equal(s.T(), event.Name, updatedUserSignup.Labels[crtapi.SocialEventUserSignupLabelKey])
	})

	s.Run("verification failed", func() {

		s.Run("too many attempts", func() {
			// given
			userSignup := testusersignup.NewUserSignup(
				testusersignup.VerificationRequiredAgo(time.Second),                                                                    // just signed up
				testusersignup.WithVerificationAttempts(configuration.GetRegistrationServiceConfig().Verification().AttemptsAllowed()), // already reached max attempts
			)
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)
			ctrl := controller.NewSignup(application)
			handler := gin.HandlerFunc(ctrl.VerifyActivationCodeHandler)

			// when
			rr := initActivationCodeVerification(s.T(), handler, userSignup.Name, "invalid")

			// then
			require.Equal(s.T(), http.StatusTooManyRequests, rr.Code) // should be `Forbidden` as in other cases?
			updatedUserSignup := &crtapi.UserSignup{}
			err := fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), updatedUserSignup)
			require.NoError(s.T(), err)
			require.True(s.T(), states.VerificationRequired(updatedUserSignup))
			require.Equal(s.T(), "3", updatedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		})

		s.Run("invalid code", func() {
			// given
			userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second)) // just signed up
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)
			ctrl := controller.NewSignup(application)
			handler := gin.HandlerFunc(ctrl.VerifyActivationCodeHandler)

			// when
			rr := initActivationCodeVerification(s.T(), handler, userSignup.Name, "invalid")

			// then
			require.Equal(s.T(), http.StatusForbidden, rr.Code)
			updatedUserSignup := &crtapi.UserSignup{}
			err := fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), updatedUserSignup)
			require.NoError(s.T(), err)
			require.True(s.T(), states.VerificationRequired(updatedUserSignup))
			require.Equal(s.T(), "1", updatedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		})

		s.Run("inactive code", func() {
			// given
			userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second)) // just signed up
			event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithStartTime(time.Now().Add(60*time.Minute)))
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)
			ctrl := controller.NewSignup(application)
			handler := gin.HandlerFunc(ctrl.VerifyActivationCodeHandler)

			// when
			rr := initActivationCodeVerification(s.T(), handler, userSignup.Name, "invalid")

			// then
			// Check the status code is what we expect.
			require.Equal(s.T(), http.StatusForbidden, rr.Code)
			updatedUserSignup := &crtapi.UserSignup{}
			err := fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), updatedUserSignup)
			require.NoError(s.T(), err)
			// Check that the correct UserSignup is passed into the FakeSignupService for update
			require.True(s.T(), states.VerificationRequired(updatedUserSignup))
			require.Equal(s.T(), "1", updatedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		})

		s.Run("expired code", func() {
			// given
			userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second)) // just signed up
			event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithEndTime(time.Now().Add(-1*time.Minute)))
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)
			ctrl := controller.NewSignup(application)
			handler := gin.HandlerFunc(ctrl.VerifyActivationCodeHandler)

			// when
			rr := initActivationCodeVerification(s.T(), handler, userSignup.Name, "invalid")

			// then
			// Check the status code is what we expect.
			require.Equal(s.T(), http.StatusForbidden, rr.Code)
			updatedUserSignup := &crtapi.UserSignup{}
			err := fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), updatedUserSignup)
			require.NoError(s.T(), err)
			// Check that the correct UserSignup is passed into the FakeSignupService for update
			require.True(s.T(), states.VerificationRequired(updatedUserSignup))
			require.Equal(s.T(), "1", updatedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		})

		s.Run("overbooked code", func() {
			// given
			userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second))                      // just signed up
			event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithActivationCount(10)) // same as `spec.MaxAttendees`
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)
			ctrl := controller.NewSignup(application)
			handler := gin.HandlerFunc(ctrl.VerifyActivationCodeHandler)

			// when
			rr := initActivationCodeVerification(s.T(), handler, userSignup.Name, "invalid")

			// then
			// Check the status code is what we expect.
			require.Equal(s.T(), http.StatusForbidden, rr.Code)
			updatedUserSignup := &crtapi.UserSignup{}
			err := fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), updatedUserSignup)
			require.NoError(s.T(), err)
			// Check that the correct UserSignup is passed into the FakeSignupService for update
			require.True(s.T(), states.VerificationRequired(updatedUserSignup))
			require.Equal(s.T(), "1", updatedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		})
	})
}

func initActivationCodeVerification(t *testing.T, handler gin.HandlerFunc, username, code string) *httptest.ResponseRecorder {
	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	payload := fmt.Sprintf(`{"code":"%s"}`, code)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup/verification/activation-code", bytes.NewBuffer([]byte(payload)))
	require.NoError(t, err)
	ctx.Request = req
	ctx.Set(context.UsernameKey, username)
	handler(ctx)
	return rr
}

type FakeSignupService struct {
	MockGetSignup                   func(ctx *gin.Context, userID, username string, checkUserSignupComplete bool) (*signup.Signup, error)
	MockSignup                      func(ctx *gin.Context) (*crtapi.UserSignup, error)
	MockGetUserSignupFromIdentifier func(userID, username string) (*crtapi.UserSignup, error)
	MockPhoneNumberAlreadyInUse     func(userID, username, value string) error
}

func (m *FakeSignupService) GetSignup(ctx *gin.Context, userID, username string, checkUserSignupComplete bool) (*signup.Signup, error) {
	return m.MockGetSignup(ctx, userID, username, checkUserSignupComplete)
}

func (m *FakeSignupService) Signup(ctx *gin.Context) (*crtapi.UserSignup, error) {
	return m.MockSignup(ctx)
}

func (m *FakeSignupService) GetUserSignupFromIdentifier(userID, username string) (*crtapi.UserSignup, error) {
	return m.MockGetUserSignupFromIdentifier(userID, username)
}

func (m *FakeSignupService) PhoneNumberAlreadyInUse(userID, username, e164phoneNumber string) error {
	return m.MockPhoneNumberAlreadyInUse(userID, username, e164phoneNumber)
}
