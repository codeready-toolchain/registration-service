package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"k8s.io/apimachinery/pkg/labels"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/labstack/echo/v4"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSpaceListRequest(t *testing.T) {
	tests := map[string]struct {
		username         string
		expectedResponse string
		expectedErr      string
	}{
		"valid space list request": {
			username:         "john.smith",
			expectedResponse: "",
			expectedErr:      "",
		},
		// "invalid space list request": {
		// username:         "john.smith",
		// expectedResponse: "",
		// expectedErr:      "",
		// },
		// "no spaces found": {
		// username:         "john.smith",
		// expectedResponse: "",
		// expectedErr:      "",
		// },
	}

	for k, tc := range tests {
		t.Run(k, func(t *testing.T) {
			//given
			johnsignup := fake.Signup("john.smith", &signup.Signup{
				CompliantUsername: "john_smith",
				Username:          "john.smith",
				Status: signup.Status{
					Ready: true,
				},
			})
			s := &handlers.SpaceLister{
				GetSignupFunc:          fake.NewSignupService(johnsignup).GetSignupFromInformer,
				GetInformerServiceFunc: fakeInformerService,
			}
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)
			ctx.Set(context.UsernameKey, tc.username)
			// ctx.SetParamNames("workspace")
			// ctx.SetParamValues("mycoolworkspace")

			// when
			err := s.HandleSpaceListRequest(ctx)

			// then
			if tc.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr)
			}
			assert.Equal(t, tc.expectedResponse, rec.Body.String())
		})
	}
}

func fakeInformerService() service.InformerService {
	inf := fake.NewFakeInformer()
	inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
		switch name {
		case "mycoolworkspace":
			return fake.NewSpace("member-1", name), nil
		case "secondworkspace":
			return fake.NewSpace("member-2", name), nil
		}
		return nil, fmt.Errorf("space not found error")
	}
	inf.ListSpaceBindingFunc = func(req ...labels.Requirement) ([]*toolchainv1alpha1.SpaceBinding, error) {
		return []*toolchainv1alpha1.SpaceBinding{
			fake.NewSpaceBinding("member-1", "john_smith", "mycoolworkspace"),
			fake.NewSpaceBinding("member-2", "john_smith", "secondworkspace"),
		}, nil
	}
	return inf
}
