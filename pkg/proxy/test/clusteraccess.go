package test

import (
	commoncluster "github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewGetMembersFunc(fakeClient client.Client) commoncluster.GetMemberClustersFunc {
	return func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
		return []*commoncluster.CachedToolchainCluster{
			{
				Config: &commoncluster.Config{
					Name:        "member-1",
					Type:        commoncluster.Member,
					APIEndpoint: "https://api.endpoint.member-1.com:6443",
					RestConfig:  &rest.Config{},
				},
				Client: fakeClient,
			},
			{
				Config: &commoncluster.Config{
					Name:              "member-2",
					APIEndpoint:       "https://api.endpoint.member-2.com:6443",
					Type:              commoncluster.Member,
					OperatorNamespace: "member-operator",
					RestConfig: &rest.Config{
						BearerToken: "clusterSAToken",
					},
				},
				Client: fakeClient,
			},
		}
	}
}
