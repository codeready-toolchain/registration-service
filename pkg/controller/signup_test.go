package controller_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	errors2 "k8s.io/apimachinery/pkg/api/errors"

	"github.com/codeready-toolchain/registration-service/pkg/verification"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/gofrs/uuid"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestSignupSuite struct {
	test.UnitTestSuite
}

func TestRunSignupSuite(t *testing.T) {
	suite.Run(t, &TestSignupSuite{test.UnitTestSuite{}})
}

func (s *TestSignupSuite) TestSignupPostHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create a mock SignupService
	svc := &FakeSignupService{}

	// Create a mock VerificationService
	verifySvc := &FakeVerificationService{}

	// Create signup instance.
	signupCtrl := controller.NewSignup(s.Config, svc, verifySvc)
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

		svc.MockCreateUserSignup = func(ctx *gin.Context) (*crtapi.UserSignup, error) {
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

		svc.MockCreateUserSignup = func(ctx *gin.Context) (*crtapi.UserSignup, error) {
			return nil, errors.New("blah")
		}

		handler(ctx)

		// Check the error is what we expect.
		test.AssertError(s.T(), rr, http.StatusInternalServerError, "blah", "error creating UserSignup resource")
	})
}

func (s *TestSignupSuite) TestSignupGetHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	// Create a mock SignupService
	svc := &FakeSignupService{}

	// Create a mock VerificationService
	verifyService := &FakeVerificationService{}

	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	// Create Signup controller instance.
	ctrl := controller.NewSignup(s.Config, svc, verifyService)
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

func (s *TestSignupSuite) TestVerifyCodeHandler() {
	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	s.Run("verification successful", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return &crtapi.UserSignup{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: userID,
						Annotations: map[string]string{
							crtapi.UserSignupUserEmailAnnotationKey:        "jsmith@redhat.com",
							crtapi.UserVerificationAttemptsAnnotationKey:   "0",
							crtapi.UserSignupVerificationCodeAnnotationKey: "999888",
							crtapi.UserVerificationExpiryAnnotationKey:     time.Now().Add(10 * time.Second).Format(verification.TimestampLayout),
						},
					},
					Spec:   crtapi.UserSignupSpec{},
					Status: crtapi.UserSignupStatus{},
				}, nil
			},
			MockUpdateUserSignup: func(userSignup *crtapi.UserSignup) (userSignup2 *crtapi.UserSignup, e error) {
				return userSignup, nil
			},
		}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(ctx *gin.Context, signup *crtapi.UserSignup, code string) error {
				return nil
			},
		}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req, err := http.NewRequest(http.MethodGet, "/api/v1/signup/verification/999888", nil)
		require.NoError(s.T(), err)

		ctx.Request = req
		ctx.Set(context.SubKey, userID)
		ctx.Params = append(ctx.Params, gin.Param{
			Key:   "code",
			Value: "999888",
		})

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)
	})

	s.Run("getsignup returns error", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return nil, errors.New("no user")
			},
		}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req, err := http.NewRequest(http.MethodGet, "/api/v1/signup/verification/111233", nil)
		require.NoError(s.T(), err)

		ctx.Request = req
		ctx.Set(context.SubKey, userID)
		ctx.Params = append(ctx.Params, gin.Param{
			Key:   "code",
			Value: "111233",
		})

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	})

	s.Run("getsignup returns nil", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return nil, nil
			},
		}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req, err := http.NewRequest(http.MethodGet, "/api/v1/signup/verification/111233", nil)
		require.NoError(s.T(), err)

		ctx.Request = req
		ctx.Set(context.SubKey, userID)
		ctx.Params = append(ctx.Params, gin.Param{
			Key:   "code",
			Value: "111233",
		})

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusNotFound, rr.Code)
	})

	s.Run("update usersignup returns error", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return &crtapi.UserSignup{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: userID,
						Annotations: map[string]string{
							crtapi.UserSignupUserEmailAnnotationKey:        "jsmith@redhat.com",
							crtapi.UserVerificationAttemptsAnnotationKey:   "0",
							crtapi.UserSignupVerificationCodeAnnotationKey: "555555",
							crtapi.UserVerificationExpiryAnnotationKey:     time.Now().Add(10 * time.Second).Format(verification.TimestampLayout),
						},
					},
					Spec:   crtapi.UserSignupSpec{},
					Status: crtapi.UserSignupStatus{},
				}, nil
			},
			MockUpdateUserSignup: func(userSignup *crtapi.UserSignup) (userSignup2 *crtapi.UserSignup, e error) {
				return nil, errors.New("error updating")
			},
		}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(ctx *gin.Context, signup *crtapi.UserSignup, code string) error {
				return nil
			},
		}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req, err := http.NewRequest(http.MethodGet, "/api/v1/signup/verification/555555", nil)
		require.NoError(s.T(), err)

		ctx.Request = req
		ctx.Set(context.SubKey, userID)
		ctx.Params = append(ctx.Params, gin.Param{
			Key:   "code",
			Value: "555555",
		})

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	})

	s.Run("verifycode returns status error", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return &crtapi.UserSignup{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: userID,
						Annotations: map[string]string{
							crtapi.UserSignupUserEmailAnnotationKey:        "jsmith@redhat.com",
							crtapi.UserVerificationAttemptsAnnotationKey:   "0",
							crtapi.UserSignupVerificationCodeAnnotationKey: "333333",
							crtapi.UserVerificationExpiryAnnotationKey:     time.Now().Add(10 * time.Second).Format(verification.TimestampLayout),
						},
					},
					Spec:   crtapi.UserSignupSpec{},
					Status: crtapi.UserSignupStatus{},
				}, nil
			},
			MockUpdateUserSignup: func(userSignup *crtapi.UserSignup) (userSignup2 *crtapi.UserSignup, e error) {
				return userSignup, nil
			},
		}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(ctx *gin.Context, signup *crtapi.UserSignup, code string) error {
				return errors2.NewTooManyRequestsError("too many requests")
			},
		}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req, err := http.NewRequest(http.MethodGet, "/api/v1/signup/verification/333333", nil)
		require.NoError(s.T(), err)

		ctx.Request = req
		ctx.Set(context.SubKey, userID)
		ctx.Params = append(ctx.Params, gin.Param{
			Key:   "code",
			Value: "333333",
		})

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusTooManyRequests, rr.Code)
	})

	s.Run("verifycode returns non status error", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return &crtapi.UserSignup{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: userID,
						Annotations: map[string]string{
							crtapi.UserSignupUserEmailAnnotationKey:        "jsmith@redhat.com",
							crtapi.UserVerificationAttemptsAnnotationKey:   "0",
							crtapi.UserSignupVerificationCodeAnnotationKey: "222222",
							crtapi.UserVerificationExpiryAnnotationKey:     time.Now().Add(10 * time.Second).Format(verification.TimestampLayout),
						},
					},
					Spec:   crtapi.UserSignupSpec{},
					Status: crtapi.UserSignupStatus{},
				}, nil
			},
			MockUpdateUserSignup: func(userSignup *crtapi.UserSignup) (userSignup2 *crtapi.UserSignup, e error) {
				return userSignup, nil
			},
		}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(ctx *gin.Context, signup *crtapi.UserSignup, code string) error {
				return errors.New("some other error")
			},
		}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req, err := http.NewRequest(http.MethodGet, "/api/v1/signup/verification/222222", nil)
		require.NoError(s.T(), err)

		ctx.Request = req
		ctx.Set(context.SubKey, userID)
		ctx.Params = append(ctx.Params, gin.Param{
			Key:   "code",
			Value: "222222",
		})

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	})

	s.Run("no code provided", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req, err := http.NewRequest(http.MethodGet, "/api/v1/signup/verification/", nil)
		require.NoError(s.T(), err)

		ctx.Request = req
		ctx.Set(context.SubKey, userID)

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusBadRequest, rr.Code)
	})
}

type FakeSignupService struct {
	MockGetSignup        func(userID string) (*signup.Signup, error)
	MockCreateUserSignup func(ctx *gin.Context) (*crtapi.UserSignup, error)
	MockGetUserSignup    func(userID string) (*crtapi.UserSignup, error)
	MockUpdateUserSignup func(userSignup *crtapi.UserSignup) (*crtapi.UserSignup, error)
}

func (m *FakeSignupService) GetSignup(userID string) (*signup.Signup, error) {
	return m.MockGetSignup(userID)
}

func (m *FakeSignupService) CreateUserSignup(ctx *gin.Context) (*crtapi.UserSignup, error) {
	return m.MockCreateUserSignup(ctx)
}

func (m *FakeSignupService) GetUserSignup(userID string) (*crtapi.UserSignup, error) {
	return m.MockGetUserSignup(userID)
}

func (m *FakeSignupService) UpdateUserSignup(userSignup *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	return m.MockUpdateUserSignup(userSignup)
}

type FakeVerificationService struct {
	MockSendVerification func(ctx *gin.Context, signup *crtapi.UserSignup) error
	MockVerifyCode       func(ctx *gin.Context, signup *crtapi.UserSignup, code string) error
}

func (m *FakeVerificationService) SendVerification(ctx *gin.Context, signup *crtapi.UserSignup) error {
	return m.MockSendVerification(ctx, signup)
}

func (m *FakeVerificationService) VerifyCode(ctx *gin.Context, signup *crtapi.UserSignup, code string) error {
	return m.MockVerifyCode(ctx, signup, code)
}
