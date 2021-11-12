package auth

import (
	"errors"
	"sync"
)

// DefaultTokenParserConfiguration represents a partition of the configuration
// that is used for configuring the default TokenParser.
type DefaultTokenParserConfiguration interface {
	GetAuthClientPublicKeysURL() string
	GetEnvironment() string
}

var (
	initDefaultTokenParserOnce = &sync.Once{}

	defaultKeyManagerHolder  *KeyManager
	defaultTokenParserHolder *TokenParser
)

// InitializeDefaultKeyManager creates the default key manager if it has not created yet.
func initializeDefaultKeyManager() (*KeyManager, error) {
	if defaultKeyManagerHolder == nil {
		var err error
		defaultKeyManagerHolder, err = NewKeyManager()
		if err != nil {
			return nil, err
		}
	}
	return defaultKeyManagerHolder, nil
}

// defaultKeyManager returns the existing KeyManager instance.
func defaultKeyManager() (*KeyManager, error) { //nolint:unparam
	if defaultKeyManagerHolder == nil {
		return nil, errors.New("no default KeyManager created, call `InitializeDefaultKeyManager()` first")
	}
	return defaultKeyManagerHolder, nil
}

// InitializeDefaultTokenParser creates the default token parser if it has not created yet.
// This function must be called in main to make sure the default parser is created during service startup.
// It will try to create the default parser only once even if called multiple times.
func InitializeDefaultTokenParser() (*TokenParser, error) {
	var returnErr error
	initDefaultTokenParserOnce.Do(func() {
		keyManager, err := initializeDefaultKeyManager()
		if err != nil {
			returnErr = err
			return
		}
		defaultTokenParserHolder, returnErr = NewTokenParser(keyManager)
	})
	if returnErr != nil {
		return nil, returnErr
	}
	return defaultTokenParserHolder, nil
}

// DefaultTokenParser returns the existing TokenManager instance.
func DefaultTokenParser() (*TokenParser, error) { //nolint:unparam
	if defaultTokenParserHolder == nil {
		return nil, errors.New("no default TokenParser created, call `InitializeDefaultTokenParser()` first")
	}
	return defaultTokenParserHolder, nil
}
