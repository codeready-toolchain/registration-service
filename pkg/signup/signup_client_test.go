package signup_test

import (
	"context"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest/fake"
	"testing"

	testclient "k8s.io/client-go/kubernetes/fake"
)

type signupClientTest struct {
	suite.Suite
	//testCtx *framework.TestCtx
}

func TestRunSignupClientTest(t *testing.T) {
	suite.Run(t, &signupClientTest{})
}

func (s *signupClientTest) SetupSuite() {

}

func (s *signupClientTest) TestDownTest() {
	//s.testCtx.Cleanup()
}

func TestCreateUserSignup(t *testing.T) {
	c := newTestSignupClient(t)

	err := c.CreateUserSignup(context.Background(), "john.smith@redhat.com", "abcde12345")
	require.NoError(t, err)

}

func newTestSignupClient(t *testing.T, initObjs ...runtime.Object) signup.SignupClient {
	client, err := signup.NewSignupClient()
	require.NoError(t, err)

	testclient.NewSimpleClientset()
	fake.CreateHTTPClient()
	fakeClient := test.NewFakeClient(t, initObjs...)
	client.Client = fakeClient

	return client
}
