package platform

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

const keychainService = "io.github.599yongyang.larder.sync"

func SetKeychainSecret(account, secret string) error {
	if strings.TrimSpace(account) == "" {
		return errors.New("credential account is required")
	}
	return keyring.Set(keychainService, account, secret)
}

func GetKeychainSecret(account string) (string, error) {
	if strings.TrimSpace(account) == "" {
		return "", errors.New("credential account is required")
	}
	secret, err := keyring.Get(keychainService, account)
	if err != nil {
		return "", errors.New("credential is not available")
	}
	return secret, nil
}

func DeleteKeychainSecret(account string) error {
	if strings.TrimSpace(account) == "" {
		return errors.New("credential account is required")
	}
	return keyring.Delete(keychainService, account)
}
