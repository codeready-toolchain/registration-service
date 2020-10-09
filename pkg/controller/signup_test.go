package controller_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/verification/service"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	errs "github.com/codeready-toolchain/registration-service/pkg/errors"
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
}

func TestRunSignupSuite(t *testing.T) {
	suite.Run(t, &TestSignupSuite{test.UnitTestSuite{}})
}

func (s *TestSignupSuite) TestSignupPostHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	svc := &FakeSignupService{}
	s.Application.MockSignupService(svc)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create signup instance.
	signupCtrl := controller.NewSignup(s.Application, s.Config)
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
	s.Application.MockSignupService(svc)

	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	// Create Signup controller instance.
	ctrl := controller.NewSignup(s.Application, s.Config)
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

func (s *TestSignupSuite) TestInitVerificationHandler() {
	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	/*var storedUserID string
	var storedUserSignup *crtapi.UserSignup
	// Create a mock SignupService
	svc := &FakeSignupService{
		MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
			storedUserID = userID
			storedUserSignup = &crtapi.UserSignup{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name: userID,
					Annotations: map[string]string{
						crtapi.UserSignupUserEmailAnnotationKey:           "jsmith@redhat.com",
						crtapi.UserSignupVerificationCounterAnnotationKey: "0",
						crtapi.UserSignupVerificationCodeAnnotationKey:    "",
					},
					Labels: map[string]string{},
				},
				Spec: crtapi.UserSignupSpec{
					VerificationRequired: true,
				},
				Status: crtapi.UserSignupStatus{},
			}
			return storedUserSignup, nil
		},
		MockUpdateUserSignup: func(userSignup *crtapi.UserSignup) (userSignup2 *crtapi.UserSignup, e error) {
			return userSignup, nil
		},
		MockPhoneNumberAlreadyInUse: func(userID, e164phoneNumber string) error {
			return nil
		},
	}*/

	//s.Application.MockSignupService(svc)

	// Create a mock VerificationService
	/*var storedVerificationCode string
	var verificationInitTimeStamp string
	verifyService := &FakeVerificationService{
		MockInitVerification: func(ctx *gin.Context, userID, e164phoneNumber string) error {
			now := time.Now()

			signup := storedUserSignup

			// If 24 hours has passed since the verification timestamp, then reset the timestamp and verification attempts
			ts, err := time.Parse("2006-01-02T15:04:05.000Z07:00", signup.Annotations[crtapi.UserSignupVerificationInitTimestampAnnotationKey])
			if err != nil || (err == nil && now.After(ts.Add(24*time.Hour))) {
				// Set a new timestamp
				signup.Annotations[crtapi.UserSignupVerificationInitTimestampAnnotationKey] = now.Format("2006-01-02T15:04:05.000Z07:00")
				verificationInitTimeStamp = signup.Annotations[crtapi.UserSignupVerificationInitTimestampAnnotationKey]
			}

			annotationCounter := signup.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey]
			counter := 0
			if annotationCounter != "" {
				counter, err = strconv.Atoi(annotationCounter)
				if err != nil {
					return errors2.NewInternalError(err)
				}
			}

			// check if counter has exceeded the limit of daily limit - if at limit error out
			if counter >= s.Config.GetVerificationDailyLimit() {
				return errors2.NewForbidden(schema.GroupResource{}, "forbidden", errors.New("daily limit has been exceeded"))
			}

			// generate verification code
			rand.Seed(time.Now().UnixNano())
			randomNum := rand.Intn(9999-1000) + 1000
			storedVerificationCode = strconv.Itoa(randomNum)
			signup.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(counter + 1)
			signup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey] = storedVerificationCode
			signup.Labels[crtapi.UserSignupUserPhoneHashLabelKey] = e164phoneNumber
			storedUserSignup = signup
			return nil
		},
	}*/

	//s.Application.MockVerificationService(verifyService)

	userSignup := &crtapi.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID,
			Namespace: s.Config.GetNamespace(),
			Annotations: map[string]string{
				crtapi.UserSignupUserEmailAnnotationKey:           "jsmith@redhat.com",
				crtapi.UserSignupVerificationCounterAnnotationKey: "0",
				crtapi.UserSignupVerificationCodeAnnotationKey:    "",
			},
			Labels: map[string]string{},
		},
		Spec: crtapi.UserSignupSpec{
			VerificationRequired: true,
		},
		Status: crtapi.UserSignupStatus{},
	}

	err = s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	// Create Signup controller instance.
	ctrl := controller.NewSignup(s.Application, s.Config)
	handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

	s.Run("init verification success", func() {
		data := []byte(`{"phone_number": "2268213044", "country_code": "1"}`)
		rr := setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")
		require.Equal(s.T(), http.StatusNoContent, rr.Code)

		updatedUserSignup, err := s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), updatedUserSignup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.NotEmpty(s.T(), updatedUserSignup.Annotations[crtapi.UserSignupVerificationInitTimestampAnnotationKey])
		require.Equal(s.T(), "1", updatedUserSignup.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
		require.Equal(s.T(), "+12268213044", updatedUserSignup.Labels[crtapi.UserSignupUserPhoneHashLabelKey])
	})

	s.Run("init verification success phone number with parenthesis and spaces", func() {
		data := []byte(`{"phone_number": "(226) 821 3044", "country_code": "1"}`)
		/*rr :=*/ setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		//assertVerification(s.T(), updatedUserSignup, http.StatusNoContent, rr.Code, userID, "12268213044",
		//	"jsmith@redhat.com", "1", storedVerificationCode, verificationInitTimeStamp)
	})

	s.Run("init verification success phone number with dashes", func() {
		data := []byte(`{"phone_number": "226-821-3044", "country_code": "1"}`)
		/*rr :=*/ setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		//assertVerification(s.T(), storedUserSignup, http.StatusNoContent, rr.Code, userID, "12268213044",
		//	"jsmith@redhat.com", "1", storedVerificationCode, verificationInitTimeStamp)
	})
	s.Run("init verification success phone number with spaces", func() {
		data := []byte(`{"phone_number": "2 2 6 8 2 1 3 0 4 4", "country_code": "1"}`)
		rr := setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusNoContent, rr.Code, "handler returned wrong status code")

		//assertVerification(s.T(), storedUserSignup, http.StatusNoContent, rr.Code, userID, "12268213044",
		//	"jsmith@redhat.com", "1", storedVerificationCode, verificationInitTimeStamp)
	})
	s.Run("init verification success country code with parenthesis", func() {
		data := []byte(`{"phone_number": "2268213044", "country_code": "(1)"}`)
		rr := setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusNoContent, rr.Code, "handler returned wrong status code")

		//assertVerification(s.T(), storedUserSignup, http.StatusNoContent, rr.Code, userID, "12268213044",
		//	"jsmith@redhat.com", "1", storedVerificationCode, verificationInitTimeStamp)
	})
	s.Run("init verification request body could not be read", func() {
		data := []byte(`{"test_number": "2268213044", "test_code": "1"}`)
		rr := setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code, "handler returned wrong status code")

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		/*require.Equal(s.T(), storedUserID, storedUserSignup.Name)
		require.Equal(s.T(), "jsmith@redhat.com", storedUserSignup.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "0", storedUserSignup.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
		require.Equal(s.T(), "", storedUserSignup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
		require.Equal(s.T(), "", storedUserSignup.Annotations[crtapi.UserSignupUserPhoneHashLabelKey])
		require.Equal(s.T(), "", storedUserSignup.Annotations[crtapi.UserSignupVerificationInitTimestampAnnotationKey])*/
	})

	s.Run("init verification daily limit exceeded", func() {
		key := configuration.EnvPrefix + "_" + "VERIFICATION_DAILY_LIMIT"
		err := os.Setenv(key, "0")
		require.NoError(s.T(), err)
		defer os.Unsetenv(key)

		data := []byte(`{"phone_number": "2268213044", "country_code": "1"}`)
		rr := setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusForbidden, rr.Code, "handler returned wrong status code")

		// Check that the correct UserSignup is passed into the FakeSignupService for update
		/*require.Equal(s.T(), storedUserID, storedUserSignup.Name)
		require.Equal(s.T(), "jsmith@redhat.com", storedUserSignup.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "0", storedUserSignup.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
		require.Equal(s.T(), verificationInitTimeStamp, storedUserSignup.Annotations[crtapi.UserSignupVerificationInitTimestampAnnotationKey])*/
	})

	s.Run("init verification handler fails when verification not required", func() {
		// Create UserSignup
		ob, err := uuid.NewV4()
		require.NoError(s.T(), err)
		userID := ob.String()

		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return &crtapi.UserSignup{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: userID,
						Annotations: map[string]string{
							crtapi.UserSignupUserEmailAnnotationKey: "jsmith@redhat.com",
						},
						Labels: map[string]string{},
					},
					Spec: crtapi.UserSignupSpec{
						VerificationRequired: false,
					},
					Status: crtapi.UserSignupStatus{},
				}, nil
			},

			MockPhoneNumberAlreadyInUse: func(userID, e164phoneNumber string) error {
				return nil
			},
		}

		s.Application.MockSignupService(svc)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

		data := []byte(`{"phone_number": "2268213044", "country_code": "1"}`)
		rr := setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	})

	s.Run("init verification handler fails when invalid phone number provided", func() {
		// Create UserSignup
		ob, err := uuid.NewV4()
		require.NoError(s.T(), err)
		userID := ob.String()

		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return &crtapi.UserSignup{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: userID,
						Annotations: map[string]string{
							crtapi.UserSignupUserEmailAnnotationKey: "jsmith@redhat.com",
						},
						Labels: map[string]string{},
					},
					Spec: crtapi.UserSignupSpec{
						VerificationRequired: true,
					},
					Status: crtapi.UserSignupStatus{},
				}, nil
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
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.InitVerificationHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		data := []byte(`{"phone_number": "!226abc8213044", "country_code": "1"}`)
		rr := setup(s.T(), handler, gin.Param{}, data, userID, http.MethodPut, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
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

		expiryTimestamp := time.Now().Add(10 * time.Second).Format(service.TimestampLayout)

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

		s.Application.MockSignupService(svc)

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(ctx *gin.Context, userID, code string) error {
				storedVerifySignup.Annotations["handled"] = "true"
				return nil
			},
		}

		s.Application.MockVerificationService(verifyService)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "999888",
		}
		rr := setup(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification")

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
	})

	s.Run("getsignup returns error", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return nil, errors.New("no user")
			},
		}

		s.Application.MockSignupService(svc)

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{}

		s.Application.MockVerificationService(verifyService)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "111233",
		}
		rr := setup(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	})

	s.Run("getsignup returns nil", func() {
		// Create a mock SignupService
		svc := &FakeSignupService{
			MockGetUserSignup: func(userID string) (userSignup *crtapi.UserSignup, e error) {
				return nil, errors2.NewNotFound(schema.GroupResource{}, userID)
			},
		}

		s.Application.MockSignupService(svc)

		// Create Signup controller instance and handle the verification request
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "111233",
		}
		rr := setup(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/111233")

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
							crtapi.UserVerificationExpiryAnnotationKey:     time.Now().Add(10 * time.Second).Format(service.TimestampLayout),
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

		s.Application.MockSignupService(svc)

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(ctx *gin.Context, userID, code string) error {
				return nil
			},
		}

		s.Application.MockVerificationService(verifyService)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "555555",
		}
		rr := setup(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/555555")

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
							crtapi.UserVerificationExpiryAnnotationKey:     time.Now().Add(10 * time.Second).Format(service.TimestampLayout),
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

		s.Application.MockSignupService(svc)

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(ctx *gin.Context, userID, code string) error {
				return errs.NewTooManyRequestsError("too many verification attempts", "")
			},
		}

		s.Application.MockVerificationService(verifyService)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "333333",
		}
		rr := setup(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/333333")

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
							crtapi.UserVerificationExpiryAnnotationKey:     time.Now().Add(10 * time.Second).Format(service.TimestampLayout),
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

		s.Application.MockSignupService(svc)

		// Create a mock VerificationService
		verifyService := &FakeVerificationService{
			MockVerifyCode: func(ctx *gin.Context, userID, code string) error {
				return errors.New("some other error")
			},
		}

		s.Application.MockVerificationService(verifyService)

		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "222222",
		}
		rr := setup(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/222222")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusInternalServerError, rr.Code)

		bodyParams := make(map[string]interface{})
		err := json.Unmarshal(rr.Body.Bytes(), &bodyParams)
		require.NoError(s.T(), err)
		require.Equal(s.T(), "Internal Server Error", bodyParams["status"])
		require.Equal(s.T(), float64(500), bodyParams["code"])
		require.Equal(s.T(), "some other error", bodyParams["message"])
		require.Equal(s.T(), "unexpected error while verifying code", bodyParams["details"])
	})

	s.Run("no code provided", func() {
		// Create Signup controller instance.
		ctrl := controller.NewSignup(s.Application, s.Config)
		handler := gin.HandlerFunc(ctrl.VerifyCodeHandler)

		param := gin.Param{
			Key:   "code",
			Value: "",
		}
		rr := setup(s.T(), handler, param, nil, userID, http.MethodGet, "/api/v1/signup/verification/")

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusBadRequest, rr.Code)
	})
}

func setup(t *testing.T, handler gin.HandlerFunc, params gin.Param, data []byte, userID, httpMethod, url string) *httptest.ResponseRecorder {
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

func assertVerification(t *testing.T, storedUserSignup *crtapi.UserSignup, expectedHTTPResponseCode int,
	actualResponseCode int, userID, expectedPhoneNumber, expectedEmail, expectedVerificationCount,
	storedVerificationCode, verificationInitTimeStamp string) {
	// Check the status code is what we expect.
	assert.Equal(t, expectedHTTPResponseCode, actualResponseCode, "handler returned wrong status code")

	// Check that the correct UserSignup is passed into the FakeSignupService for update
	require.Equal(t, userID, storedUserSignup.Name)
	require.Equal(t, expectedEmail, storedUserSignup.Annotations[crtapi.UserSignupUserEmailAnnotationKey])
	require.Equal(t, expectedVerificationCount, storedUserSignup.Annotations[crtapi.UserSignupVerificationCounterAnnotationKey])
	require.Equal(t, storedVerificationCode, storedUserSignup.Annotations[crtapi.UserSignupVerificationCodeAnnotationKey])
	require.Equal(t, verificationInitTimeStamp, storedUserSignup.Annotations[crtapi.UserSignupVerificationInitTimestampAnnotationKey])
	require.Equal(t, expectedPhoneNumber, storedUserSignup.Labels[crtapi.UserSignupUserPhoneHashLabelKey])
}

type FakeSignupService struct {
	MockGetSignup               func(userID string) (*signup.Signup, error)
	MockCreateUserSignup        func(ctx *gin.Context) (*crtapi.UserSignup, error)
	MockGetUserSignup           func(userID string) (*crtapi.UserSignup, error)
	MockUpdateUserSignup        func(userSignup *crtapi.UserSignup) (*crtapi.UserSignup, error)
	MockPhoneNumberAlreadyInUse func(userID, e164phoneNumber string) error
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

func (m *FakeSignupService) PhoneNumberAlreadyInUse(userID, e164phoneNumber string) error {
	return m.MockPhoneNumberAlreadyInUse(userID, e164phoneNumber)
}

type FakeVerificationService struct {
	MockInitVerification func(ctx *gin.Context, userID, e164phoneNumber string) error
	MockVerifyCode       func(ctx *gin.Context, userID, code string) error
}

func (m *FakeVerificationService) InitVerification(ctx *gin.Context, userID, e164phoneNumber string) error {
	return m.MockInitVerification(ctx, userID, e164phoneNumber)
}

func (m *FakeVerificationService) VerifyCode(ctx *gin.Context, userID, code string) error {
	return m.MockVerifyCode(ctx, userID, code)
}
