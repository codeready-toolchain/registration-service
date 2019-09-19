package auth

import (
	"errors"
	"log"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

var defaultKeyManager *KeyManager

// DefaultKeyManagerWithConfig creates the default manager if it has not created yet.
// This function must be called in main to make sure the default manager is created during service startup.
// It will try to create the default manager only once even if called multiple times.
func DefaultKeyManagerWithConfig(logger *log.Logger, config *configuration.Registry) (*KeyManager, error) {
	var err error
	if defaultKeyManager == nil {
		defaultKeyManager, err = NewKeyManager(logger, config)
		return defaultKeyManager, err
	}
	return defaultKeyManager, nil
}

// DefaultKeyManager returns the existing KeyManager instance.
func DefaultKeyManager() (*KeyManager, error) {
	if defaultKeyManager == nil {
		return nil, errors.New("no default KeyManager created, call `DefaultKeyManagerWithConfig()` first")
	}
	return defaultKeyManager, nil
}
