package auth

import (
	"errors"
	"log"
	"sync"
)

// DefaultTokenParserConfiguration represents a partition of the configuration
// that is used for configuring the default TokenParser.
type DefaultTokenParserConfiguration interface {
	GetAuthClientPublicKeysURL() string
}

var muKM sync.Mutex
var muTP sync.Mutex

var defaultKeyManagerHolder *KeyManager
var defaultTokenParserHolder *TokenParser

// InitializeDefaultKeyManager creates the default key manager if it has not created yet.
// This function must be called in main to make sure the default manager is created during service startup.
// It will try to create the default manager only once even if called multiple times.
func initializeDefaultKeyManager(logger *log.Logger, config KeyManagerConfiguration) (*KeyManager, error) {
	muKM.Lock()
	defer muKM.Unlock()
	if defaultKeyManagerHolder == nil {
		var err error
		defaultKeyManagerHolder, err = NewKeyManager(logger, config)
		if err != nil {
			return nil, err
		}
		return defaultKeyManagerHolder, nil
	}
	return nil, errors.New("default KeyManager can be created only once")
}

// defaultKeyManager returns the existing KeyManager instance.
func defaultKeyManager() (*KeyManager, error) {
	if defaultKeyManagerHolder == nil {
		return nil, errors.New("no default KeyManager created, call `InitializeDefaultKeyManager()` first")
	}
	return defaultKeyManagerHolder, nil
}

// InitializeDefaultTokenParser creates the default token parser if it has not created yet.
// This function must be called in main to make sure the default parser is created during service startup.
// It will try to create the default parser only once even if called multiple times.
func InitializeDefaultTokenParser(logger *log.Logger, config DefaultTokenParserConfiguration) (*TokenParser, error) {
	muTP.Lock()
	defer muTP.Unlock()
	if defaultTokenParserHolder == nil {
		var err error
		keyManager, err := initializeDefaultKeyManager(logger, config)
		if err != nil {
			return nil, err
		}
		defaultTokenParserHolder, err = NewTokenParser(logger, keyManager)
		if err != nil {
			return nil, err
		}
		return defaultTokenParserHolder, nil
	}
	return nil, errors.New("default TokenParser can be created only once")
}

// DefaultTokenParser returns the existing TokenManager instance.
func DefaultTokenParser() (*TokenParser, error) {
	if defaultTokenParserHolder == nil {
		return nil, errors.New("no default TokenParser created, call `InitializeDefaultTokenParser()` first")
	}
	return defaultTokenParserHolder, nil
}
