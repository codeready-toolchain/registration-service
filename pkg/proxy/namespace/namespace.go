package namespace

import (
	"context"
	"fmt"
	"net/url"

	"github.com/codeready-toolchain/registration-service/pkg/log"

	authenticationv1 "k8s.io/api/authentication/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceAccess holds an information needed to access the namespace in a member cluster for the specific user
type NamespaceAccess struct { // nolint:revive
	// APIURL is the Cluster API Endpoint for the namespace
	APIURL url.URL
	// SAToken is a token of the Service Account which represents the user in the namespace
	SAToken string
	// client is the kube client for the cluster.
	client client.Client
}

func NewNamespaceAccess(apiURL url.URL, saToken string, client client.Client) *NamespaceAccess {
	return &NamespaceAccess{
		APIURL:  apiURL,
		SAToken: saToken,
		client:  client,
	}
}

// Validate returns true if the given token is valid
func (a *NamespaceAccess) Validate() (bool, error) {
	tr := &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{
			Token: a.SAToken,
		},
	}
	if err := a.client.Create(context.TODO(), tr); err != nil {
		return false, err
	}

	log.Info(nil, fmt.Sprintf("TokenReview status: %v", tr.Status))
	return tr.Status.Authenticated, nil
}
