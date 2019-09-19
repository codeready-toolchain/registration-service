package signup

import (
	"context"
)

type SignupService interface {
	CreateUserSignup(ctx context.Context, username, userID string) error
}

type signupClientImpl struct {
	Clientset apiextension.Interface
}
