package signup

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"gopkg.in/square/go-jose.v2"
)

// PublicKey represents an RSA public key with a Key ID
type PublicKey struct {
	KeyID string
	Key   *rsa.PublicKey
}

// JSONKeys the remote keys encoded in a json document
type JSONKeys struct {
	Keys []interface{} `json:"keys"`
}

type KeyManager struct {
	config *configuration.Registry
	logger *log.Logger
	keyMap map[string]*rsa.PublicKey
}

// NewKeyManager creates a new KeyManager and retrieves the public keys from the given URL.
func NewKeyManager(logger *log.Logger, config *configuration.Registry) (*KeyManager, error) {
	keysEndpointURL := config.GetAuthClientPublicKeysURL()
	km := &KeyManager{
		logger: logger,
		config: config,
	}
	// fetch raw keys
	keys, err := km.fetchKeys(keysEndpointURL)
	if err != nil {
		return nil, err
	}
	// add them to the kid map
	for _, key := range keys {
		km.keyMap[key.KeyID] = key.Key
	}
	return km, nil
}

// Key retrieves the public key for a given kid.
func (km *KeyManager) Key(kid string) (*rsa.PublicKey, error) {
	key, ok := km.keyMap[kid]
	if !ok {
		return nil, errors.New("unknown kid")
	}	
	return key, nil
}

// unmarshalKeys unmarshals keys from given JSON.
func (km *KeyManager) unmarshalKeys(jsonData []byte) ([]*PublicKey, error) {
	var keys []*PublicKey
	var raw JSONKeys
	err := json.Unmarshal(jsonData, &raw)
	if err != nil {
		return nil, err
	}
	for _, key := range raw.Keys {
		jsonKeyData, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		publicKey, err := km.unmarshalKey(jsonKeyData)
		if err != nil {
			return nil, err
		}
		keys = append(keys, publicKey)
	}
	return keys, nil
}

// unmarshalKey unmarshals a single key from a given JSON.
func (km *KeyManager) unmarshalKey(jsonData []byte) (*PublicKey, error) {
	key := &jose.JSONWebKey{}
	err := key.UnmarshalJSON(jsonData)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.Key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("Key is not an *rsa.PublicKey")
	}
	return &PublicKey{key.KeyID, rsaKey}, nil
}

// fetchKeys fetches the keys from the given URL, unmarshalling them.
func (km *KeyManager) fetchKeys(keysEndpointURL string) ([]*PublicKey, error) {
	// use httpClient to perform request
	httpClient := http.DefaultClient
	req, err := http.NewRequest("GET", keysEndpointURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// cleanup and close after being done
	defer func() {
		ioutil.ReadAll(res.Body)
		if err != io.EOF {
			km.logger.Println("failed read remaining data before closing response")
		}
		err := res.Body.Close()
		if err != nil {
			km.logger.Println("failed to close response after reading")
		}
	}()
	// read and parse response body
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		return nil, err
	}
	bodyString := buf.String()
	// if status code was not OK, bail out
	if res.StatusCode != http.StatusOK {
		km.logger.Println(map[string]interface{}{
			"response_status": res.Status,
			"response_body":   bodyString,
			"url":             keysEndpointURL,
		}, "unable to obtain public keys from remote service")
		return nil, errors.New("unable to obtain public keys from remote service")
	}
	// unmarshal the keys
	keys, err := km.unmarshalKeys([]byte(bodyString))
	if err != nil {
		return nil, err
	}
	km.logger.Println(map[string]interface{}{
		"url":            keysEndpointURL,
		"number_of_keys": len(keys),
	}, "public keys loaded")
	// return the retrieved keys
	return keys, nil
}
