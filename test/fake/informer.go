package fake

import (
	"context"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	spacetest "github.com/codeready-toolchain/toolchain-common/pkg/test/space"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func NewFakeInformer() Informer {
	return Informer{}
}

type Informer struct {
	GetMurFunc                 func(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetSpaceFunc               func(name string) (*toolchainv1alpha1.Space, error)
	GetToolchainStatusFunc     func() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignupFunc          func(name string) (*toolchainv1alpha1.UserSignup, error)
	ListSpaceBindingFunc       func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error)
	GetProxyPluginConfigFunc   func(name string) (*toolchainv1alpha1.ProxyPlugin, error)
	GetNSTemplateTierFunc      func(name string) (*toolchainv1alpha1.NSTemplateTier, error)
	ListBannedUsersByEmailFunc func(email string) ([]toolchainv1alpha1.BannedUser, error)
}

func (f Informer) GetProxyPluginConfig(name string) (*toolchainv1alpha1.ProxyPlugin, error) {
	if f.GetProxyPluginConfigFunc != nil {
		return f.GetProxyPluginConfigFunc(name)
	}
	panic("not supposed to call GetProxyPluginConfig")
}

func (f Informer) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	if f.GetMurFunc != nil {
		return f.GetMurFunc(name)
	}
	panic("not supposed to call GetMasterUserRecord")
}

func (f Informer) GetSpace(name string) (*toolchainv1alpha1.Space, error) {
	if f.GetSpaceFunc != nil {
		return f.GetSpaceFunc(name)
	}
	panic("not supposed to call GetSpace")
}

func (f Informer) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	if f.GetToolchainStatusFunc != nil {
		return f.GetToolchainStatusFunc()
	}
	panic("not supposed to call GetToolchainStatus")
}

func (f Informer) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	if f.GetUserSignupFunc != nil {
		return f.GetUserSignupFunc(name)
	}
	panic("not supposed to call GetUserSignup")
}

func (f Informer) ListSpaceBindings(req ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
	if f.ListSpaceBindingFunc != nil {
		return f.ListSpaceBindingFunc(req...)
	}
	panic("not supposed to call ListSpaceBindings")
}

func (f Informer) GetNSTemplateTier(tier string) (*toolchainv1alpha1.NSTemplateTier, error) {
	if f.GetNSTemplateTierFunc != nil {
		return f.GetNSTemplateTierFunc(tier)
	}
	panic("not supposed to call GetNSTemplateTierFunc")
}

func (f Informer) ListBannedUsersByEmail(email string) ([]toolchainv1alpha1.BannedUser, error) {
	if f.ListBannedUsersByEmailFunc != nil {
		return f.ListBannedUsersByEmailFunc(email)
	}
	panic("not supposed to call BannedUsersByEmail")
}

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
	return spacetest.NewSpace(configuration.Namespace(), name,
		spaceTestOptions...,
	)
}

func NewSpaceBinding(name, murLabelValue, spaceLabelValue, role string) *toolchainv1alpha1.SpaceBinding {
	return &toolchainv1alpha1.SpaceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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
			Namespace: configuration.Namespace(),
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

type InformerServiceOptions func(informer *Informer)

func WithGetNSTemplateTierFunc(getNsTemplateTierFunc func(tier string) (*toolchainv1alpha1.NSTemplateTier, error)) InformerServiceOptions {
	return func(informer *Informer) {
		informer.GetNSTemplateTierFunc = getNsTemplateTierFunc
	}
}

func WithListSpaceBindingFunc(listSpaceBindingFunc func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error)) InformerServiceOptions {
	return func(informer *Informer) {
		informer.ListSpaceBindingFunc = listSpaceBindingFunc
	}
}

func WithGetSpaceFunc(getSpaceFunc func(name string) (*toolchainv1alpha1.Space, error)) InformerServiceOptions {
	return func(informer *Informer) {
		informer.GetSpaceFunc = getSpaceFunc
	}
}

func WithGetMurFunc(getMurFunc func(name string) (*toolchainv1alpha1.MasterUserRecord, error)) InformerServiceOptions {
	return func(informer *Informer) {
		informer.GetMurFunc = getMurFunc
	}
}

func WithGetBannedUsersByEmailFunc(bannedUsersByEmail func(ermail string) ([]toolchainv1alpha1.BannedUser, error)) InformerServiceOptions {
	return func(informer *Informer) {
		informer.ListBannedUsersByEmailFunc = bannedUsersByEmail
	}
}

func GetInformerService(fakeClient client.Client, options ...InformerServiceOptions) func() service.InformerService {
	return func() service.InformerService {

		inf := NewFakeInformer()
		inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
			space := &toolchainv1alpha1.Space{}
			err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: configuration.Namespace()}, space)
			return space, err
		}
		inf.ListSpaceBindingFunc = func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			sbList := &toolchainv1alpha1.SpaceBindingList{}
			err := fakeClient.List(context.TODO(), sbList, &client.ListOptions{LabelSelector: labels.NewSelector().Add(reqs...)})
			return sbList.Items, err
		}
		inf.GetNSTemplateTierFunc = func(tier string) (*toolchainv1alpha1.NSTemplateTier, error) {
			nsTemplateTier := &toolchainv1alpha1.NSTemplateTier{}
			err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: tier, Namespace: configuration.Namespace()}, nsTemplateTier)
			return nsTemplateTier, err
		}
		inf.GetMurFunc = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
			mur := &toolchainv1alpha1.MasterUserRecord{}
			err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: configuration.Namespace()}, mur)
			return mur, err
		}

		for _, modify := range options {
			modify(&inf)
		}

		return inf
	}
}

func InitClient(t *testing.T, initObjs ...runtime.Object) *test.FakeClient {
	scheme := runtime.NewScheme()
	var AddToSchemes runtime.SchemeBuilder
	addToSchemes := append(AddToSchemes,
		toolchainv1alpha1.AddToScheme)
	err := addToSchemes.AddToScheme(scheme)
	require.NoError(t, err)
	cl := test.NewFakeClient(t, initObjs...)
	return cl
}
