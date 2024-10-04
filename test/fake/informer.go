package fake

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	spacetest "github.com/codeready-toolchain/toolchain-common/pkg/test/space"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewSpace(name, targetCluster, compliantUserName string, spaceTestOptions ...spacetest.Option) *toolchainv1alpha1.Space {

	spaceTestOptions = append(spaceTestOptions,
		spacetest.WithLabel(toolchainv1alpha1.SpaceCreatorLabelKey, compliantUserName),
		spacetest.WithSpecTargetCluster(targetCluster),
		spacetest.WithStatusTargetCluster(targetCluster),
		spacetest.WithTierName("base1ns"),
		spacetest.WithStatusProvisionedNamespaces(
			[]toolchainv1alpha1.SpaceNamespace{
				{
					Name: name + "-dev",
					Type: "default",
				},
				{
					Name: name + "-stage",
				},
			},
		),
	)
	return spacetest.NewSpace(test.HostOperatorNs, name,
		spaceTestOptions...,
	)
}

func NewSpaceBinding(name, murLabelValue, spaceLabelValue, role string) *toolchainv1alpha1.SpaceBinding {
	return &toolchainv1alpha1.SpaceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: test.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey: murLabelValue,
				toolchainv1alpha1.SpaceBindingSpaceLabelKey:            spaceLabelValue,
			},
		},
		Spec: toolchainv1alpha1.SpaceBindingSpec{
			SpaceRole:        role,
			MasterUserRecord: murLabelValue,
			Space:            spaceLabelValue,
		},
	}
}

func NewBase1NSTemplateTier() *toolchainv1alpha1.NSTemplateTier {
	return &toolchainv1alpha1.NSTemplateTier{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: test.HostOperatorNs,
			Name:      "base1ns",
		},
		Spec: toolchainv1alpha1.NSTemplateTierSpec{
			ClusterResources: &toolchainv1alpha1.NSTemplateTierClusterResources{
				TemplateRef: "basic-clusterresources-123456new",
			},
			Namespaces: []toolchainv1alpha1.NSTemplateTierNamespace{
				{
					TemplateRef: "basic-dev-123456new",
				},
				{
					TemplateRef: "basic-stage-123456new",
				},
			},
			SpaceRoles: map[string]toolchainv1alpha1.NSTemplateTierSpaceRole{
				"admin": {
					TemplateRef: "basic-admin-123456new",
				},
				"viewer": {
					TemplateRef: "basic-viewer-123456new",
				},
			},
		},
	}
}

func NewMasterUserRecord(name string) *toolchainv1alpha1.MasterUserRecord {
	return &toolchainv1alpha1.MasterUserRecord{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: configuration.Namespace(),
		},
		Spec: toolchainv1alpha1.MasterUserRecordSpec{
			UserAccounts: []toolchainv1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
		},
		Status: toolchainv1alpha1.MasterUserRecordStatus{
			Conditions: []toolchainv1alpha1.Condition{
				{
					Type:   toolchainv1alpha1.MasterUserRecordReady,
					Status: "blah-blah-blah",
				},
			},
		},
	}
}

func NewBannedUser(name, email string) *toolchainv1alpha1.BannedUser {
	return &toolchainv1alpha1.BannedUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: hash.EncodeString(email),
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: email,
		},
	}
}
