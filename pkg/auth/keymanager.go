package auth

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	stderrors "errors"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"

	"gopkg.in/square/go-jose.v2"
)

// KeyManagerConfiguration represents a partition of the configuration
// that is used for configuring the KeyManager.
type KeyManagerConfiguration interface {
	GetAuthClientPublicKeysURL() string
	GetEnvironment() string
}

// PublicKey represents an RSA public key with a Key ID
type PublicKey struct {
	KeyID string
	Key   *rsa.PublicKey
}

// JSONKeys the remote keys encoded in a json document
type JSONKeys struct {
	Keys []interface{} `json:"keys"`
}

// KeyManager manages the public keys for token validation.
type KeyManager struct {
	keyMap map[string]*rsa.PublicKey
}

// NewKeyManager creates a new KeyManager and retrieves the public keys from the given URL.
func NewKeyManager() (*KeyManager, error) {
	cfg := configuration.GetRegistrationServiceConfig()
	keysEndpointURL := cfg.Auth().AuthClientPublicKeysURL()
	km := &KeyManager{
		keyMap: make(map[string]*rsa.PublicKey),
	}
	// fetch raw keys
	if keysEndpointURL != "" {
		if cfg.Environment() == "e2e-tests" {
			log.Infof(nil, "fetching e2e public keys")
			keys := authsupport.GetE2ETestPublicKey()

			// add them to the kid map
			for _, key := range keys {
				km.keyMap[key.KeyID] = key.Key
			}
		} else {
			log.Infof(nil, "fetching public keys from url: %s", keysEndpointURL)
			keys, err := km.fetchKeys(keysEndpointURL)
			if err != nil {
				return nil, err
			}
			// add them to the kid map
			for _, key := range keys {
				km.keyMap[key.KeyID] = key.Key
			}
		}
	} else {
		log.Info(nil, "no public key url given, not fetching keys")
	}
	return km, nil
}

// Key retrieves the public key for a given kid.
func (km *KeyManager) Key(kid string) (*rsa.PublicKey, error) {
	key, ok := km.keyMap[kid]
	if !ok {
		return nil, stderrors.New("unknown kid")
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
		return nil, stderrors.New("Key is not an *rsa.PublicKey")
	}
	return &PublicKey{key.KeyID, rsaKey}, nil
}

// unmarshalls the keys from a byte array.
func (km *KeyManager) fetchKeysFromBytes(keysBytes []byte) ([]*PublicKey, error) {
	keys, err := km.unmarshalKeys(keysBytes)
	if err != nil {
		return nil, err
	}

	log.Infof(nil, "%v public keys loaded", string(keysBytes))
	// return the retrieved keys
	return keys, nil
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
		_, err := ioutil.ReadAll(res.Body)
		if stderrors.Is(err, io.EOF) {
			log.Error(nil, err, "failed read remaining data before closing response")
		}
		err = res.Body.Close()
		if err != nil {
			log.Error(nil, err, "failed to close response after reading")
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
		err := stderrors.New("unable to obtain public keys from remote service")
		log.WithValues(map[string]interface{}{
			"response_status": res.Status,
			"response_body":   bodyString,
			"keys_url":        keysEndpointURL,
		}).Error(nil, err, "")
		return nil, err
	}
	// unmarshal the keys
	return km.fetchKeysFromBytes([]byte(bodyString))
}
