package auth

import (
	"errors"
	"log"
	"sync"
)

var mu sync.Mutex

var defaultKeyManager *KeyManager

// InitializeDefaultKeyManager creates the default manager if it has not created yet.
// This function must be called in main to make sure the default manager is created during service startup.
// It will try to create the default manager only once even if called multiple times.
func InitializeDefaultKeyManager(logger *log.Logger, config KeyManagerConfiguration) (*KeyManager, error) {
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
		return nil, errors.New("no default KeyManager created, call `InitializeDefaultKeyManager()` first")
	}
	return defaultKeyManager, nil
}

// not exported, only used for test, removes singleton.
func resetDefaultKeyManager() {
	defaultKeyManager = nil
}