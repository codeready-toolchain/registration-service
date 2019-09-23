package auth

import (
	"errors"
	"log"
	"sync"
)

var muKM sync.Mutex
var muTP sync.Mutex

var defaultKeyManager *KeyManager
var defaultTokenParser *TokenParser

// InitializeDefaultKeyManager creates the default key manager if it has not created yet.
// This function must be called in main to make sure the default manager is created during service startup.
// It will try to create the default manager only once even if called multiple times.
func InitializeDefaultKeyManager(logger *log.Logger, config KeyManagerConfiguration) (*KeyManager, error) {
	muKM.Lock()
	defer muKM.Unlock()
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

// InitializeDefaultTokenParser creates the default token parser if it has not created yet.
// This function must be called in main to make sure the default parser is created during service startup.
// It will try to create the default parser only once even if called multiple times.
func InitializeDefaultTokenParser(logger *log.Logger, keyManager *KeyManager) (*TokenParser, error) {
	muTP.Lock()
	defer muTP.Unlock()
	if defaultTokenParser == nil {
		var err error
		defaultTokenParser, err = NewTokenParser(logger, keyManager)
		if err != nil {
			return nil, err
		}
		return defaultTokenParser, nil
	}
	return nil, errors.New("default TokenParser can be created only once")
}

// DefaultTokenParser returns the existing TokenManager instance.
func DefaultTokenParser() (*TokenParser, error) {
	if defaultTokenParser == nil {
		return nil, errors.New("no default TokenParser created, call `InitializeDefaultTokenParser()` first")
	}
	return defaultTokenParser, nil
}
