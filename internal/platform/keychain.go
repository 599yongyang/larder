package platform

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

const keychainService = "io.github.599yongyang.larder.sync"

func SetKeychainSecret(account, secret string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("credential storage is only available on macOS")
	}
	if strings.TrimSpace(account) == "" {
		return errors.New("credential account is required")
	}
	cmd := exec.Command("security", "add-generic-password", "-a", account, "-s", keychainService, "-w", secret, "-U")
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(output)))
	}
	return nil
}

func GetKeychainSecret(account string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("credential storage is only available on macOS")
	}
	cmd := exec.Command("security", "find-generic-password", "-a", account, "-s", keychainService, "-w")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.New("credential is not available")
	}
	return strings.TrimSpace(string(output)), nil
}

func DeleteKeychainSecret(account string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("credential storage is only available on macOS")
	}
	cmd := exec.Command("security", "delete-generic-password", "-a", account, "-s", keychainService)
	if err := cmd.Run(); err != nil {
		return errors.New("credential could not be removed")
	}
	return nil
}
