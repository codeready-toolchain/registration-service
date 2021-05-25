package controller_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/codeready-toolchain/toolchain-common/pkg/states"

	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	verification_service "github.com/codeready-toolchain/registration-service/pkg/verification/service"
	"gopkg.in/h2non/gock.v1"

	"github.com/codeready-toolchain/registration-service/pkg/verification/service"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apiv1 "k8s.io/api/core/v1"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestSignupSuite struct {
	test.UnitTestSuite
	httpClient *http.Client
}

func TestRunSignupSuite(t *testing.T) {
	suite.Run(t, &TestSignupSuite{test.UnitTestSuite{}, nil})
}

func (s *TestSignupSuite) SetHTTPClientFactoryOption() {
	s.httpClient = &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(s.httpClient)

	serviceOption := func(svc *verification_service.ServiceImpl) {
		svc.HTTPClient = s.httpClient
	}

	opt := func(serviceFactory *factory.ServiceFactory) {
		serviceFactory.WithVerificationServiceOption(serviceOption)
	}

	s.WithFactoryOption(opt)
}

func (s *TestSignupSuite) TestSignupPostHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	svc := &FakeSignupService{}
	s.Application.MockSignupService(svc)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config().IsTestingMode(), "testing mode not set correctly to true")

	// Create signup instance.
	signupCtrl := controller.NewSignup(s.Application, s.Config())
	handler := gin.HandlerFunc(signupCtrl.PostHandler)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	s.Run("signup created", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Put userID to the context
		ob, err := uuid.NewV4()
		require.NoError(s.T(), err)
		expectedUserID := ob.String()
		ctx.Set(context.SubKey, expectedUserID)
		ctx.Set(context.EmailKey, expectedUserID+"@test.com")
		email := ctx.GetString(context.EmailKey)
		signup := &crtapi.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      userID.String(),
				Namespace: "namespace-foo",
				Annotations: map[string]string{
					crtapi.UserSignupUserEmailAnnotationKey: email,
				},
			},
			Spec: crtapi.UserSignupSpec{
				Username: "bill",
			},
			Status: crtapi.UserSignupStatus{
				Conditions: []crtapi.Condition{
					{
						Type:    crtapi.UserSignupComplete,
						Status:  apiv1.ConditionFalse,
						Reason:  "test_reason",
						Message: "test_message",
					},
				},
			},
		}

		svc.MockSignup = func(ctx *gin.Context) (*crtapi.UserSignup, error) {
			assert.Equal(s.T(), expectedUserID, ctx.GetString(context.SubKey))
			assert.Equal(s.T(), expectedUserID+"@test.com", ctx.GetString(context.EmailKey))
			return signup, nil
		}
		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusAccepted, rr.Code)
	})

	s.Run("signup error", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		svc.MockSignup = func(ctx *gin.Context) (*crtapi.UserSignup, error) {
			return nil, errors.New("blah")
		}

		handler(ctx)

		// Check the error is what we expect.
		test.AssertError(s.T(), rr, http.StatusInternalServerError, "blah", "error creating UserSignup resource")
	})

	s.Run("signup forbidden error", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		svc.MockSignup = func(ctx *gin.Context) (*crtapi.UserSignup, error) {
			return nil, errors2.NewForbidden(schema.GroupResource{}, "", errors.New("forbidden test error"))
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
	s.Application.MockSignupService(svc)

	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	// Create Signup controller instance.
	ctrl := controller.NewSignup(s.Application, s.Config())
	handler := gin.HandlerFunc(ctrl.GetHandler)

	s.Run("signups found", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.SubKey, userID)

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
		svc.MockGetSignup = func(id string) (*signup.Signup, error) {
			if id == userID {
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
		ctx.Set(context.SubKey, userID)

		svc.MockGetSignup = func(id string) (*signup.Signup, error) {
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
		ctx.Set(context.SubKey, userID)

		svc.MockGetSignup = func(id string) (*signup.Signup, error) {
			return nil, errors.New("oopsie woopsie")
		}

		handler(ctx)

		// Check the error is what we expect.
		test.AssertError(s.T(), rr, http.StatusInternalServerError, "oopsie woopsie", "error getting UserSignup resource")
	})
}

func (s *TestSignupSuite) TestInitVerificationHandler() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHTTPClientFactoryOption()

	defer gock.Off()
	s.OverrideConfig(s.DefaultConfig())

	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	userSignup := &crtapi.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID,
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				crtapi.UserSignupUserEmailAnnotationKey:           "jsmith@redhat.com",
				crtapi.UserSignupVerificationCounterAnnotationKey: "0",
				crtapi.UserSignupVerificationCodeAnnotationKey:    "",
			},
			Labels: map[string]string{},
		},
		Spec:   crtapi.UserSignupSpec{},
		Status: crtapi.UserSignupStatus{},
	}
	states.SetVerificationRequired(userSignup, true)

	err = s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	// Create Signup controller instance.
	ctrl := controller.NewSignup(s.Application, s.Config())
	handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

	assertInitVerificationSuccess := func(phoneNumber, expectedHash string, expectedCounter int) {
		gock.New("https://api.twilio.com").
			Reply(http.StatusNoContent).
			BodyString("")

		data := []byte(fmt.Sprintf(`{"phone_number": "%s", "country_code": "1"}`, phoneNumber))
		rr := initVerification(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")
		require.Equal(s.T(), http.StatusNoContent, rr.Code)

		updatedUserSignup, err := s.FakeUserSignupClient.Get(userSignup.Name)
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
		rr := initVerification(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")
		require.Equal(s.T(), http.StatusBadRequest, rr.Code)

		bodyParams := make(map[string]interface{})
		err = json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Bad Request", bodyParams["status"])
		require.Equal(s.T(), float64(400), bodyParams["code"])
		require.Equal(s.T(), "strconv.Atoi: parsing \"(1)\": invalid syntax", bodyParams["message"])
		require.Equal(s.T(), "invalid country_code", bodyParams["details"])
	})
	s.Run("init verification request body could not be read", func() {
		data := []byte(`{"test_number": "2268213044", "test_code": "1"}`)
		rr := initVerification(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code)

		bodyParams := make(map[string]interface{})
		err = json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		messageLines := strings.Split(bodyParams["message"].(string), "\n")
		require.Equal(s.T(), "Bad Request", bodyParams["status"])
		require.Equal(s.T(), float64(400), bodyParams["code"])
		require.Contains(s.T(), messageLines, "Key: 'Phone.CountryCode' Error:Field validation for 'CountryCode' failed on the 'required' tag")
		require.Contains(s.T(), messageLines, "Key: 'Phone.PhoneNumber' Error:Field validation for 'PhoneNumber' failed on the 'required' tag")
		require.Equal(s.T(), "error reading request body", bodyParams["details"])
	})

	s.Run("init verification daily limit exceeded", func() {
		key := configuration.EnvPrefix + "_" + "VERIFICATION_DAILY_LIMIT"
		err := os.Setenv(key, "0")
		require.NoError(s.T(), err)
		defer os.Unsetenv(key)

		data := []byte(`{"phone_number": "2268213044", "country_code": "1"}`)
		rr := initVerification(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusForbidden, rr.Code, "handler returned wrong status code")
	})

	s.Run("init verification handler fails when verification not required", func() {
		// Create UserSignup
		ob, err := uuid.NewV4()
		require.NoError(s.T(), err)
		userID := ob.String()

		userSignup := &crtapi.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      userID,
				Namespace: s.Config().GetNamespace(),
				Annotations: map[string]string{
					crtapi.UserSignupUserEmailAnnotationKey: "jsmith@redhat.com",
				},
			},
			Spec:   crtapi.UserSignupSpec{},
			Status: crtapi.UserSignupStatus{},
		}
		states.SetVerificationRequired(userSignup, false)

		err = s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config())
		handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

		data := []byte(`{"phone_number": "2268213044", "country_code": "1"}`)
		rr := initVerification(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code)

		bodyParams := make(map[string]interface{})
		err = json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Bad Request", bodyParams["status"])
		require.Equal(s.T(), float64(400), bodyParams["code"])
		require.Equal(s.T(), "forbidden request:verification code will not be sent", bodyParams["message"])
		require.Equal(s.T(), "forbidden request", bodyParams["details"])
	})

	s.Run("init verification handler fails when invalid phone number provided", func() {
		// Create UserSignup
		ob, err := uuid.NewV4()
		require.NoError(s.T(), err)
		userID := ob.String()

		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				us := crtapi.UserSignup{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: userID,
						Annotations: map[string]string{
							crtapi.UserSignupUserEmailAnnotationKey: "jsmith@redhat.com",
						},
						Labels: map[string]string{},
					},
					Spec:   crtapi.UserSignupSpec{},
					Status: crtapi.UserSignupStatus{},
				}
				states.SetVerificationRequired(&us, true)
				return &us, nil
			},
			MockUpdateUserSignup: func(userSignup *crtapi.UserSignup) (userSignup2 *crtapi.UserSignup, e error) {
				return userSignup, nil
			},
			MockPhoneNumberAlreadyInUse: func(userID, e164phoneNumber string) error {
				return nil
			},
		}

		s.Application.MockSignupService(svc)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config())
		handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		data := []byte(`{"phone_number": "!226%213044", "country_code": "1"}`)
		rr := initVerification(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	})
}

func (s *TestSignupSuite) TestVerifyCodeHandler() {
	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	userSignup := &crtapi.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID,
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				crtapi.UserSignupUserEmailAnnotationKey:        "jsmith@redhat.com",
				crtapi.UserVerificationAttemptsAnnotationKey:   "0",
				crtapi.UserSignupVerificationCodeAnnotationKey: "999888",
				crtapi.UserVerificationExpiryAnnotationKey:     time.Now().Add(10 * time.Second).Format(service.TimestampLayout),
			},
		},
		Spec:   crtapi.UserSignupSpec{},
		Status: crtapi.UserSignupStatus{},
	}

	err = s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	s.Run("verification successful", func() {
		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config())
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "999888",
		}
		rr := initVerification(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		updatedUserSignup, err := s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		require.False(s.T(), states.VerificationRequired(updatedUserSignup))
		require.Empty(s.T(), updatedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		require.Empty(s.T(), updatedUserSignup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Empty(s.T(), updatedUserSignup.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
	})

	s.Run("getsignup returns error", func() {
		// Simulate returning an error
		s.FakeUserSignupClient.MockGet = func(string) (userSignup *crtapi.UserSignup, e error) {
			return nil, errors.New("no user")
		}
		defer func() { s.FakeUserSignupClient.MockGet = nil }()

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config())
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "111233",
		}
		rr := initVerification(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)

		bodyParams := make(map[string]interface{})
		err = json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Internal Server Error", bodyParams["status"])
		require.Equal(s.T(), float64(500), bodyParams["code"])
		require.Equal(s.T(), fmt.Sprintf("no user:error retrieving usersignup: %s", userSignup.Name), bodyParams["message"])
		require.Equal(s.T(), "error while verifying code", bodyParams["details"])
	})

	s.Run("getsignup returns nil", func() {

		s.FakeUserSignupClient.MockGet = func(userID string) (userSignup *crtapi.UserSignup, e error) {
			return nil, errors2.NewNotFound(schema.GroupResource{}, userID)
		}
		defer func() { s.FakeUserSignupClient.MockGet = nil }()

		// Create Signup controller instance and handle the verification request
		ctrl := controller.NewSignup(s.Application, s.Config())
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "111233",
		}
		rr := initVerification(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/111233")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusNotFound, rr.Code)

		bodyParams := make(map[string]interface{})
		err = json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Not Found", bodyParams["status"])
		require.Equal(s.T(), float64(404), bodyParams["code"])
		require.Equal(s.T(), fmt.Sprintf(" \"%s\" not found:user not found", userSignup.Name), bodyParams["message"])
		require.Equal(s.T(), "error while verifying code", bodyParams["details"])
	})

	s.Run("update usersignup returns error", func() {
		s.FakeUserSignupClient.MockUpdate = func(*crtapi.UserSignup) (*crtapi.UserSignup, error) {
			return nil, errors2.NewServiceUnavailable("service unavailable")
		}
		defer func() { s.FakeUserSignupClient.MockUpdate = nil }()

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config())
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "555555",
		}
		rr := initVerification(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/555555")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)

		bodyParams := make(map[string]interface{})
		err = json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Internal Server Error", bodyParams["status"])
		require.Equal(s.T(), float64(500), bodyParams["code"])
		require.Equal(s.T(), "service unavailable:error updating UserSignup", bodyParams["message"])
		require.Equal(s.T(), "error while verifying code", bodyParams["details"])
	})

	s.Run("verifycode returns status error", func() {

		userSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey] = "9999"
		userSignup.Annotations[crtapi.UserVerificationExpiryAnnotationKey] = time.Now().Add(10 * time.Second).Format(service.TimestampLayout)
		userSignup.Annotations[crtapi.UserSignupVerificationTimestampAnnotationKey] = time.Now().Format(service.TimestampLayout)

		err := s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		require.NoError(s.T(), err)
		err = s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config())
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "333333",
		}
		rr := initVerification(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/333333")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusTooManyRequests, rr.Code)

		bodyParams := make(map[string]interface{})
		err = json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "Too Many Requests", bodyParams["status"])
		require.Equal(s.T(), float64(429), bodyParams["code"])
		require.Equal(s.T(), "too many verification attempts:", bodyParams["message"])
		require.Equal(s.T(), "error while verifying code", bodyParams["details"])
	})

	s.Run("no code provided", func() {
		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config())
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "",
		}
		rr := initVerification(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusBadRequest, rr.Code)
	})
}

func initVerification(t *testing.T, handler gin.HandlerFunc, params gin.Param, data []byte, userID, httpMethod, url string) *httptest.ResponseRecorder {
	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	req, err := http.NewRequest(httpMethod, url, bytes.NewBuffer(data))
	require.NoError(t, err)
	ctx.Request = req
	ctx.Set(context.SubKey, userID)

	ctx.Params = append(ctx.Params, params)
	handler(ctx)

	return rr
}

type FakeSignupService struct {
	MockGetSignup               func(userID string) (*signup.Signup, error)
	MockSignup                  func(ctx *gin.Context) (*crtapi.UserSignup, error)
	MockGetUserSignup           func(userID string) (*crtapi.UserSignup, error)
	MockUpdateUserSignup        func(userSignup *crtapi.UserSignup) (*crtapi.UserSignup, error)
	MockPhoneNumberAlreadyInUse func(userID, value string) error
}

func (m *FakeSignupService) GetSignup(userID string) (*signup.Signup, error) {
	return m.MockGetSignup(userID)
}

func (m *FakeSignupService) Signup(ctx *gin.Context) (*crtapi.UserSignup, error) {
	return m.MockSignup(ctx)
}

func (m *FakeSignupService) GetUserSignup(userID string) (*crtapi.UserSignup, error) {
	return m.MockGetUserSignup(userID)
}

func (m *FakeSignupService) UpdateUserSignup(userSignup *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	return m.MockUpdateUserSignup(userSignup)
}

func (m *FakeSignupService) PhoneNumberAlreadyInUse(userID, e164phoneNumber string) error {
	return m.MockPhoneNumberAlreadyInUse(userID, e164phoneNumber)
}
