package auth

import (
	"errors"
	"log"
	"sync"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

var mu sync.Mutex

var defaultKeyManager *KeyManager

// DefaultKeyManagerWithConfig creates the default manager if it has not created yet.
// This function must be called in main to make sure the default manager is created during service startup.
// It will try to create the default manager only once even if called multiple times.
func DefaultKeyManagerWithConfig(logger *log.Logger, config *configuration.Registry) (*KeyManager, error) {
	mu.Lock()
	defer mu.Unlock()
	if defaultKeyManager == nil {
		var err error
		defaultKeyManager, err = NewKeyManager(logger, config)
		if err != nil {
			return nil, err
		}
		return defaultKeyManager, nil
	}
	return nil, errors.New("default KeyManager can be created only once")
}

// DefaultKeyManager returns the existing KeyManager instance.
func DefaultKeyManager() (*KeyManager, error) {
	if defaultKeyManager == nil {
		return nil, errors.New("no default KeyManager created, call `DefaultKeyManagerWithConfig()` first")
	}
	return defaultKeyManager, nil
}
