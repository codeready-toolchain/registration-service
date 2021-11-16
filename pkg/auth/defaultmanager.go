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
	defaultTokenParser         *TokenParser
)

// InitializeDefaultTokenParser creates the default token parser if it has not created yet.
// This function must be called in main to make sure the default parser is created during service startup.
// It will try to create the default parser only once even if called multiple times.
func InitializeDefaultTokenParser() (*TokenParser, error) {
	var returnErr error
	initDefaultTokenParserOnce.Do(func() {
		keyManager, err := NewKeyManager()
		if err != nil {
			returnErr = err
			return
		}
		defaultTokenParser, returnErr = NewTokenParser(keyManager)
	})
	if returnErr != nil {
		return nil, returnErr
	}
	return defaultTokenParser, nil
}

// DefaultTokenParser returns the existing TokenManager instance.
func DefaultTokenParser() (*TokenParser, error) { //nolint:unparam
	if defaultTokenParser == nil {
		return nil, errors.New("no default TokenParser created, call `InitializeDefaultTokenParser()` first")
	}
	return defaultTokenParser, nil
}
