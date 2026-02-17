package keychain

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "iap-client-macos"

// KeychainService provides secure credential storage using macOS Keychain
type KeychainService struct{}

// NewKeychainService creates a new keychain service
func NewKeychainService() *KeychainService {
	return &KeychainService{}
}

// StorePassword stores a password in the keychain
func (ks *KeychainService) StorePassword(instance, username, password string) error {
	key := fmt.Sprintf("%s@%s", username, instance)
	return keyring.Set(serviceName, key, password)
}

// GetPassword retrieves a password from the keychain
func (ks *KeychainService) GetPassword(instance, username string) (string, error) {
	key := fmt.Sprintf("%s@%s", username, instance)
	return keyring.Get(serviceName, key)
}

// DeletePassword removes a password from the keychain
func (ks *KeychainService) DeletePassword(instance, username string) error {
	key := fmt.Sprintf("%s@%s", username, instance)
	return keyring.Delete(serviceName, key)
}

// HasPassword checks if a password exists in the keychain
func (ks *KeychainService) HasPassword(instance, username string) bool {
	_, err := ks.GetPassword(instance, username)
	return err == nil
}

// StoreConnectionCredentials stores credentials for a connection
func (ks *KeychainService) StoreConnectionCredentials(connectionID, username, password string) error {
	key := fmt.Sprintf("connection:%s:%s", connectionID, username)
	return keyring.Set(serviceName, key, password)
}

// GetConnectionCredentials retrieves credentials for a connection
func (ks *KeychainService) GetConnectionCredentials(connectionID, username string) (string, error) {
	key := fmt.Sprintf("connection:%s:%s", connectionID, username)
	return keyring.Get(serviceName, key)
}

// DeleteConnectionCredentials removes credentials for a connection
func (ks *KeychainService) DeleteConnectionCredentials(connectionID, username string) error {
	key := fmt.Sprintf("connection:%s:%s", connectionID, username)
	return keyring.Delete(serviceName, key)
}
