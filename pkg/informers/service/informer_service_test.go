package service_test

import (
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/informers/service"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/codeready-toolchain/toolchain-common/pkg/test/masteruserrecord"
	"github.com/codeready-toolchain/toolchain-common/pkg/test/space"
	"github.com/codeready-toolchain/toolchain-common/pkg/test/usersignup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInformerService(t *testing.T) {
	// given
	murJohn := masteruserrecord.NewMasterUserRecord(t, "johnMur")
	murNoise := masteruserrecord.NewMasterUserRecord(t, "noise")
	spaceJohn := space.NewSpace(test.HostOperatorNs, "johnSpace")
	spaceNoise := space.NewSpace(test.HostOperatorNs, "noiseSpace")
	pluginTekton := &toolchainv1alpha1.ProxyPlugin{ObjectMeta: metav1.ObjectMeta{
		Name:      "tekton-results",
		Namespace: test.HostOperatorNs,
	}}
	pluginNoise := &toolchainv1alpha1.ProxyPlugin{ObjectMeta: metav1.ObjectMeta{
		Name:      "noise",
		Namespace: test.HostOperatorNs,
	}}
	status := &toolchainv1alpha1.ToolchainStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "toolchain-status",
			Namespace: test.HostOperatorNs,
		},
		Status: toolchainv1alpha1.ToolchainStatusStatus{
			HostOperator: &toolchainv1alpha1.HostOperatorStatus{
				Version: "v1alpha1",
			},
		},
	}
	signupJohn := usersignup.NewUserSignup(usersignup.WithName("johnUserSignup"),
		usersignup.WithTargetCluster("member2"))
	signupJohn.Spec.IdentityClaims = toolchainv1alpha1.IdentityClaimsEmbedded{
		PropagatedClaims: toolchainv1alpha1.PropagatedClaims{
			Sub:         "foo",
			OriginalSub: "sub-key",
		},
		PreferredUsername: "foo@redhat.com",
		GivenName:         "Foo",
		FamilyName:        "Bar",
		Company:           "Red Hat",
	}
	signupNoise := usersignup.NewUserSignup(usersignup.WithName("noise"))
	bannedAlice := &toolchainv1alpha1.BannedUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alice",
			Namespace: test.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: hash.EncodeString("alice@redhat.com"),
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "alice@redhat.com",
		},
	}
	bannedBob := &toolchainv1alpha1.BannedUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bob",
			Namespace: test.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: hash.EncodeString("bob@redhat.com"),
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "bob@redhat.com",
		},
	}
	bannedBobDup := &toolchainv1alpha1.BannedUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bob-dup",
			Namespace: test.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: hash.EncodeString("bob@redhat.com"),
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "bob@redhat.com",
		},
	}

	client := test.NewFakeClient(t, murJohn, murNoise, spaceJohn, spaceNoise, pluginTekton,
		pluginNoise, status, signupJohn, signupNoise, bannedAlice, bannedBob, bannedBobDup)
	svc := service.NewInformerService(client, test.HostOperatorNs)

	t.Run("masteruserrecords", func(t *testing.T) {
		t.Run("not found", func(t *testing.T) {
			// when
			val, err := svc.GetMasterUserRecord("unknown")

			//then
			assert.Empty(t, val)
			assert.EqualError(t, err, "masteruserrecords.toolchain.dev.openshift.com \"unknown\" not found")
		})

		t.Run("found", func(t *testing.T) {
			// when
			val, err := svc.GetMasterUserRecord("johnMur")

			// then
			require.NotNil(t, val)
			require.NoError(t, err)
			assert.Equal(t, murJohn.Spec, val.Spec)
		})
	})

	t.Run("spaces", func(t *testing.T) {
		t.Run("not found", func(t *testing.T) {
			// when
			val, err := svc.GetSpace("unknown")

			// then
			assert.Empty(t, val)
			assert.EqualError(t, err, "spaces.toolchain.dev.openshift.com \"unknown\" not found")
		})

		t.Run("found", func(t *testing.T) {
			// when
			val, err := svc.GetSpace("johnSpace")

			// then
			require.NotNil(t, val)
			require.NoError(t, err)
			assert.Equal(t, spaceJohn.Spec, val.Spec)
		})
	})

	t.Run("proxy configs", func(t *testing.T) {
		t.Run("not found", func(t *testing.T) {
			// when
			val, err := svc.GetProxyPluginConfig("unknown")

			// then
			assert.Empty(t, val)
			assert.EqualError(t, err, "proxyplugins.toolchain.dev.openshift.com \"unknown\" not found")
		})

		t.Run("found", func(t *testing.T) {
			// when
			val, err := svc.GetProxyPluginConfig("tekton-results")

			// then
			require.NotNil(t, val)
			require.NoError(t, err)
			assert.Equal(t, pluginTekton.Spec, val.Spec)
		})
	})

	t.Run("toolchainstatuses", func(t *testing.T) {
		t.Run("not found", func(t *testing.T) {
			// given
			client := test.NewFakeClient(t)
			svc := service.NewInformerService(client, test.HostOperatorNs)

			// when
			val, err := svc.GetToolchainStatus()

			// then
			assert.Empty(t, val)
			assert.EqualError(t, err, "toolchainstatuses.toolchain.dev.openshift.com \"toolchain-status\" not found")
		})

		t.Run("found", func(t *testing.T) {
			// when
			val, err := svc.GetToolchainStatus()

			// then
			require.NotNil(t, val)
			require.NoError(t, err)
			assert.Equal(t, status.Status, val.Status)
		})
	})

	t.Run("usersignups", func(t *testing.T) {
		t.Run("not found", func(t *testing.T) {
			// when
			val, err := svc.GetUserSignup("unknown")

			// then
			assert.Empty(t, val)
			assert.EqualError(t, err, "usersignups.toolchain.dev.openshift.com \"unknown\" not found")
		})

		t.Run("found", func(t *testing.T) {
			// when
			val, err := svc.GetUserSignup("johnUserSignup")

			// then
			require.NotNil(t, val)
			require.NoError(t, err)
			assert.Equal(t, signupJohn.Spec, val.Spec)
		})
	})

	t.Run("bannedusers", func(t *testing.T) {
		t.Run("not found", func(t *testing.T) {
			// when
			rbb, err := svc.ListBannedUsersByEmail("unknown@unknown.com")

			// then
			require.NoError(t, err)
			require.Empty(t, rbb)
		})

		t.Run("invalid email", func(t *testing.T) {
			// when
			rbb, err := svc.ListBannedUsersByEmail("not-an-email")

			// then
			require.NoError(t, err)
			require.Empty(t, rbb)
		})

		t.Run("found one", func(t *testing.T) {
			// when
			rbb, err := svc.ListBannedUsersByEmail(bannedAlice.Spec.Email)

			// then
			require.NotNil(t, rbb)
			require.NoError(t, err)
			require.Len(t, rbb, 1, "expected 1 result for email %s", bannedAlice.Spec.Email)
			require.Equal(t, *bannedAlice, rbb[0])
		})

		t.Run("found multiple", func(t *testing.T) {
			// when
			rbb, err := svc.ListBannedUsersByEmail(bannedBob.Spec.Email)

			// then
			require.NotNil(t, rbb)
			require.NoError(t, err)
			require.Len(t, rbb, 2, "expected 2 results for email %s", bannedBob.Spec.Email)
			require.ElementsMatch(t, []toolchainv1alpha1.BannedUser{*bannedBob, *bannedBobDup}, rbb)
		})
	})
}
