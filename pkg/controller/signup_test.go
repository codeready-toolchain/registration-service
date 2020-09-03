package controller_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/verification"
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

func (s *TestSignupSuite) TestVerificationPostHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	data := []byte(`{"phone_number": "2268213044", "country_code": "1"}`)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup/verification", bytes.NewBuffer(data))
	require.NoError(s.T(), err)

	dataMap := make(map[string]string)
	err = json.Unmarshal(data, &dataMap)

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
	handler := gin.HandlerFunc(ctrl.UpdateVerificationHandler)

	var storedVerificationCode string
	var verificationTimeStamp string

	expiryTimestamp := time.Now().Add(10 * time.Second).Format(verification.TimestampLayout)
	expected := &crtapi.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name: userID,
			Annotations: map[string]string{
				crtapi.UserSignupUserEmailAnnotationKey:      "jsmith@redhat.com",
				crtapi.UserVerificationAttemptsAnnotationKey: "0",
				crtapi.UserVerificationExpiryAnnotationKey:   expiryTimestamp,
			},
		},
		Spec:   crtapi.UserSignupSpec{},
		Status: crtapi.UserSignupStatus{},
	}

	svc.MockUpdateWithVerificationCode = func(dailyLimit int, responseBody map[string]string, id, code string) (userSignup *crtapi.UserSignup, err error) {
		if id != userID {
			return nil, errors2.NewNotFound(schema.GroupResource{}, "not found")
		}

		annotationCounter := expected.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey]
		counter := 0
		if annotationCounter != "" {
			counter, err = strconv.Atoi(annotationCounter)
			if err != nil {
				return nil, errors2.NewInternalError(err)
			}
		}

		// check if counter has exceeded the limit of daily limit - if at limit error out
		if counter > dailyLimit {
			return nil, errors2.NewForbidden(schema.GroupResource{}, "forbidden", errors.New("daily limit has been exceeded"))
		}

		verificationTimeStamp = time.Now().Format(verification.TimestampLayout)
		expected.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(counter + 1)
		expected.Annotations[crtapi.UserSignupPhoneNumberLabelKey] = responseBody["country_code"] + responseBody["phone_number"]
		expected.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey] = code
		expected.Annotations[crtapi.UserSignupVerificationTimestampAnnotationKey] = verificationTimeStamp

		return expected, nil
	}

	svc.MockGetUserSignup = func(id string) (*crtapi.UserSignup, error) {
		if id == userID {
			return expected, nil
		}
		return nil, nil
	}

	verifyService.MockGenerateVerificationCode = func() (s string, err error) {
		rand.Seed(time.Now().UnixNano())
		storedVerificationCode = strconv.Itoa(rand.Intn(1000))
		return storedVerificationCode, nil
	}

	verifyService.MockInitVerification = func(ctx *gin.Context, signup *crtapi.UserSignup, countryCode, phoneNumber string) (*crtapi.UserSignup, error) {
		return nil, nil
	}

	s.Run("post verification success", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.SubKey, userID)

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		require.Equal(s.T(), userID, expected.Name)
		require.Equal(s.T(), "jsmith@redhat.com", expected.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "1", expected.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
		require.Equal(s.T(), storedVerificationCode, expected.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Equal(s.T(), dataMap["country_code"]+dataMap["phone_number"], expected.Annotations[crtapi.UserSignupPhoneNumberLabelKey])
		require.Equal(s.T(), expiryTimestamp, expected.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
		require.Equal(s.T(), verificationTimeStamp, expected.Annotations[crtapi.UserSignupVerificationTimestampAnnotationKey])
	})

	s.Run("post verification request body could not be read", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.SubKey, userID)

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusInternalServerError, rr.Code, "handler returned wrong status code")

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		require.Equal(s.T(), userID, expected.Name)
		require.Equal(s.T(), "jsmith@redhat.com", expected.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "1", expected.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
		require.Equal(s.T(), storedVerificationCode, expected.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Equal(s.T(), dataMap["country_code"]+dataMap["phone_number"], expected.Annotations[crtapi.UserSignupPhoneNumberLabelKey])
		require.Equal(s.T(), expiryTimestamp, expected.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
		require.Equal(s.T(), verificationTimeStamp, expected.Annotations[crtapi.UserSignupVerificationTimestampAnnotationKey])
	})

	s.Run("post verification usersignup not found", func() {
		req, err = http.NewRequest(http.MethodPost, "/api/v1/signup/verification", bytes.NewBuffer(data))
		require.NoError(s.T(), err)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.SubKey, "1111")

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusNotFound, rr.Code, "handler returned wrong status code")

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		require.Equal(s.T(), userID, expected.Name)
		require.Equal(s.T(), "jsmith@redhat.com", expected.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "1", expected.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
		require.NotEqual(s.T(), storedVerificationCode, expected.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Equal(s.T(), dataMap["country_code"]+dataMap["phone_number"], expected.Annotations[crtapi.UserSignupPhoneNumberLabelKey])
		require.Equal(s.T(), expiryTimestamp, expected.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
		require.Equal(s.T(), verificationTimeStamp, expected.Annotations[crtapi.UserSignupVerificationTimestampAnnotationKey])
	})

	s.Run("post verification daily limit exceeded", func() {
		key := configuration.EnvPrefix + "_" + "VERIFICATION_DAILY_LIMIT"
		err = os.Setenv(key, "0")
		defer os.Unsetenv(key)

		req, err = http.NewRequest(http.MethodPost, "/api/v1/signup/verification", bytes.NewBuffer(data))
		require.NoError(s.T(), err)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.SubKey, userID)

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusForbidden, rr.Code, "handler returned wrong status code")

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		require.Equal(s.T(), userID, expected.Name)
		require.Equal(s.T(), "jsmith@redhat.com", expected.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "1", expected.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
		require.NotEqual(s.T(), storedVerificationCode, expected.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Equal(s.T(), dataMap["country_code"]+dataMap["phone_number"], expected.Annotations[crtapi.UserSignupPhoneNumberLabelKey])
		require.Equal(s.T(), expiryTimestamp, expected.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
		require.Equal(s.T(), verificationTimeStamp, expected.Annotations[crtapi.UserSignupVerificationTimestampAnnotationKey])
	})
}

func (s *TestSignupSuite) TestVerifyCodeHandler() {
	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	s.Run("verification successful", func() {
		var storedUserID string
		var storedUserSignup *crtapi.UserSignup
		var storedVerifySignup *crtapi.UserSignup

		expiryTimestamp := time.Now().Add(10 * time.Second).Format(verification.TimestampLayout)

		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				storedUserID = userID
				return &crtapi.UserSignup{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: userID,
						Annotations: map[string]string{
							crtapi.UserSignupUserEmailAnnotationKey:        "jsmith@redhat.com",
							crtapi.UserVerificationAttemptsAnnotationKey:   "0",
							crtapi.UserSignupVerificationCodeAnnotationKey: "999888",
							crtapi.UserVerificationExpiryAnnotationKey:     expiryTimestamp,
						},
					},
					Spec:   crtapi.UserSignupSpec{},
					Status: crtapi.UserSignupStatus{},
				}, nil
			},
			MockUpdateUserSignup: func(userSignup *crtapi.UserSignup) (userSignup2 *crtapi.UserSignup, e error) {
				storedUserSignup = userSignup
				return userSignup, nil
			},
		}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(signup *crtapi.UserSignup, code string) (*crtapi.UserSignup, error) {
				signup.Annotations["handled"] = "true"
				storedVerifySignup = signup
				return nil, nil
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

		// Check that the correct userID is passed into the FakeSignupService
		require.Equal(s.T(), userID, storedUserID)

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		require.Equal(s.T(), userID, storedUserSignup.Name)
		require.Equal(s.T(), "jsmith@redhat.com", storedUserSignup.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "0", storedUserSignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		require.Equal(s.T(), "999888", storedUserSignup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Equal(s.T(), expiryTimestamp, storedUserSignup.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
		require.Equal(s.T(), "true", storedUserSignup.Annotations["handled"])

		// Check that the correct UserSignup is passed into the FakeVerificationService
		require.Equal(s.T(), userID, storedVerifySignup.Name)
		require.Equal(s.T(), "jsmith@redhat.com", storedVerifySignup.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "0", storedVerifySignup.Annotations[crtapi.UserVerificationAttemptsAnnotationKey])
		require.Equal(s.T(), "999888", storedVerifySignup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Equal(s.T(), expiryTimestamp, storedVerifySignup.Annotations[crtapi.UserVerificationExpiryAnnotationKey])
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

		// Create Signup controller instance and handle the verification request
		ctrl := controller.NewSignup(s.Config, svc, &FakeVerificationService{})
		rr := s.handleVerify(ctrl, userID, "111233")

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
			MockVerifyCode: func(signup *crtapi.UserSignup, code string) (*crtapi.UserSignup, error) {
				return nil, nil
			},
		}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		rr := s.handleVerify(ctrl, userID, "555555")

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
			MockVerifyCode: func(signup *crtapi.UserSignup, code string) (*crtapi.UserSignup, error) {
				return nil, errors2.NewTooManyRequestsError("too many requests")
			},
		}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		rr := s.handleVerify(ctrl, userID, "333333")

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
			MockVerifyCode: func(signup *crtapi.UserSignup, code string) (*crtapi.UserSignup, error) {
				return nil, errors.New("some other error")
			},
		}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		rr := s.handleVerify(ctrl, userID, "222222")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)
		require.Equal(s.T(), "Internal Server Error", bodyParams["status"])
		require.Equal(s.T(), float64(500), bodyParams["code"])
		require.Equal(s.T(), "some other error", bodyParams["message"])
		require.Equal(s.T(), "error while verifying code", bodyParams["details"])
	})

	s.Run("no code provided", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{}

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{}

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Config, svc, verifyService)
		rr := s.handleVerify(ctrl, userID, "")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusBadRequest, rr.Code)
	})
}

func (s *TestSignupSuite) handleVerify(controller *controller.Signup, userID, code string) *httptest.ResponseRecorder {
	handler := gin.HandlerFunc(controller.VerifyCodeHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/signup/verification/%s", code), nil)
	require.NoError(s.T(), err)

	ctx.Request = req
	ctx.Set(context.SubKey, userID)
	ctx.Params = append(ctx.Params, gin.Param{
		Key:   "code",
		Value: code,
	})

	handler(ctx)

	return rr
}

type FakeSignupService struct {
	MockGetSignup        func(userID string) (*signup.Signup, error)
	MockCreateUserSignup func(ctx *gin.Context) (*crtapi.UserSignup, error)
	MockGetUserSignup    func(userID string) (*crtapi.UserSignup, error)
	MockUpdateUserSignup func(userSignup *crtapi.UserSignup) (*crtapi.UserSignup, error)
	//MockUpdateWithVerificationCode func(dailyLimit int, responseBody map[string]string, userID, code string) (*crtapi.UserSignup, error)
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

//func (m *FakeSignupService) UpdateWithVerificationCode(dailyLimit int, responseBody map[string]string, userID, code string) (*crtapi.UserSignup, error) {
//	return m.MockUpdateWithVerificationCode(dailyLimit, responseBody, userID, code)
//}

type FakeVerificationService struct {
	MockInitVerification         func(ctx *gin.Context, signup *crtapi.UserSignup, countryCode, phoneNumber string) (*crtapi.UserSignup, error)
	MockVerifyCode               func(signup *crtapi.UserSignup, code string) (*crtapi.UserSignup, error)
	MockGenerateVerificationCode func() (string, error)
}

func (m *FakeVerificationService) InitVerification(ctx *gin.Context, signup *crtapi.UserSignup, countryCode, phoneNumber string) (*crtapi.UserSignup, error) {
	return m.MockInitVerification(ctx, signup, countryCode, phoneNumber)
}

func (m *FakeVerificationService) VerifyCode(signup *crtapi.UserSignup, code string) (*crtapi.UserSignup, error) {
	return m.MockVerifyCode(signup, code)
}

func (m *FakeVerificationService) GenerateVerificationCode() (string, error) {
	return m.MockGenerateVerificationCode()
}
