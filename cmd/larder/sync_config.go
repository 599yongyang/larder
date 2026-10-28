package larder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"larder/internal/core/remotesync"
	"larder/internal/core/transfer"
	"larder/internal/platform"
)

const webdavCredentialAccount = "webdav"
const s3CredentialAccount = "s3"
const encryptionCredentialAccount = "sync-encryption"

type SyncConfig struct {
	Provider                string `json:"provider"`
	URL                     string `json:"url"`
	Username                string `json:"username"`
	Endpoint                string `json:"endpoint"`
	Region                  string `json:"region"`
	Bucket                  string `json:"bucket"`
	Prefix                  string `json:"prefix"`
	AccessKey               string `json:"accessKey"`
	Filename                string `json:"filename"`
	PasswordSet             bool   `json:"passwordSet"`
	EncryptionEnabled       bool   `json:"encryptionEnabled"`
	EncryptionPassphraseSet bool   `json:"encryptionPassphraseSet"`
	LastRemoteHash          string `json:"lastRemoteHash"`
	LastSyncAt              string `json:"lastSyncAt"`
	LastSyncKind            string `json:"lastSyncKind"`
}

type syncConfigFile struct {
	Provider          string `json:"provider"`
	URL               string `json:"url"`
	Username          string `json:"username"`
	Endpoint          string `json:"endpoint"`
	Region            string `json:"region"`
	Bucket            string `json:"bucket"`
	Prefix            string `json:"prefix"`
	AccessKey         string `json:"accessKey"`
	Filename          string `json:"filename"`
	EncryptionEnabled bool   `json:"encryptionEnabled"`
	LastRemoteHash    string `json:"lastRemoteHash"`
	LastSyncAt        string `json:"lastSyncAt"`
	LastSyncKind      string `json:"lastSyncKind"`
}

type syncRemote interface {
	Test(context.Context) error
	Upload(context.Context, []byte) error
	Download(context.Context) ([]byte, error)
}

func (s *AppService) syncConfigPath() string {
	return filepath.Join(s.dataDir, "sync-config.json")
}

func secretsEncryptionAvailable() bool {
	_, err := platform.GetKeychainSecret(encryptionCredentialAccount)
	return err == nil
}

func secretAccount(provider string) string {
	if provider == "s3" {
		return s3CredentialAccount
	}
	return webdavCredentialAccount
}

func (s *AppService) SyncConfig() SyncConfig {
	raw, err := os.ReadFile(s.syncConfigPath())
	if err != nil {
		return SyncConfig{Provider: "webdav", Filename: "larder-data.zip", EncryptionEnabled: true}
	}
	var file syncConfigFile
	if json.Unmarshal(raw, &file) != nil {
		return SyncConfig{Provider: "webdav", Filename: "larder-data.zip", EncryptionEnabled: true}
	}
	if file.Provider == "" {
		file.Provider = "webdav"
	}
	if file.Filename == "" {
		file.Filename = "larder-data.zip"
	}
	_, secretErr := platform.GetKeychainSecret(secretAccount(file.Provider))
	return SyncConfig{Provider: file.Provider, URL: file.URL, Username: file.Username, Endpoint: file.Endpoint, Region: file.Region, Bucket: file.Bucket, Prefix: file.Prefix, AccessKey: file.AccessKey, Filename: file.Filename, PasswordSet: secretErr == nil, EncryptionEnabled: file.EncryptionEnabled, EncryptionPassphraseSet: secretsEncryptionAvailable(), LastSyncAt: file.LastSyncAt, LastSyncKind: file.LastSyncKind}
}

func (s *AppService) SaveSyncConfig(config SyncConfig, password, encryptionPassword string) error {
	if config.Provider == "" {
		config.Provider = "webdav"
	}
	if config.Provider != "webdav" && config.Provider != "s3" {
		return errors.New("unsupported sync provider")
	}
	config.Filename = strings.TrimSpace(config.Filename)
	if config.Filename == "" {
		config.Filename = "larder-data.zip"
	}
	if config.Provider == "webdav" {
		if _, err := remotesync.NewWebDAV(strings.TrimSpace(config.URL), strings.TrimSpace(config.Username), password, config.Filename); err != nil {
			return err
		}
	} else {
		if _, err := remotesync.NewS3(strings.TrimSpace(config.Endpoint), strings.TrimSpace(config.Region), strings.TrimSpace(config.Bucket), strings.TrimSpace(config.AccessKey), password, strings.TrimSpace(config.Prefix), config.Filename); err != nil {
			return err
		}
	}
	previous := syncConfigFile{}
	if raw, err := os.ReadFile(s.syncConfigPath()); err == nil {
		_ = json.Unmarshal(raw, &previous)
	}
	file := syncConfigFile{Provider: config.Provider, URL: strings.TrimSpace(config.URL), Username: strings.TrimSpace(config.Username), Endpoint: strings.TrimSpace(config.Endpoint), Region: strings.TrimSpace(config.Region), Bucket: strings.TrimSpace(config.Bucket), Prefix: strings.TrimSpace(config.Prefix), AccessKey: strings.TrimSpace(config.AccessKey), Filename: config.Filename, EncryptionEnabled: config.EncryptionEnabled, LastRemoteHash: previous.LastRemoteHash, LastSyncAt: previous.LastSyncAt, LastSyncKind: previous.LastSyncKind}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.syncConfigPath(), raw, 0o600); err != nil {
		return err
	}
	if password != "" {
		if err := platform.SetKeychainSecret(secretAccount(config.Provider), password); err != nil {
			return err
		}
	}
	if encryptionPassword != "" {
		return platform.SetKeychainSecret(encryptionCredentialAccount, encryptionPassword)
	}
	return nil
}

func (s *AppService) remoteClient() (syncRemote, error) {
	config := s.SyncConfig()
	secret, _ := platform.GetKeychainSecret(secretAccount(config.Provider))
	switch config.Provider {
	case "s3":
		return remotesync.NewS3(config.Endpoint, config.Region, config.Bucket, config.AccessKey, secret, config.Prefix, config.Filename)
	default:
		return remotesync.NewWebDAV(config.URL, config.Username, secret, config.Filename)
	}
}

func (s *AppService) recordRemoteHash(hash string) {
	file := syncConfigFile{Provider: "webdav", Filename: "larder-data.zip"}
	if raw, err := os.ReadFile(s.syncConfigPath()); err == nil {
		_ = json.Unmarshal(raw, &file)
	}
	file.LastRemoteHash = hash
	raw, err := json.MarshalIndent(file, "", "  ")
	if err == nil {
		_ = os.WriteFile(s.syncConfigPath(), raw, 0o600)
	}
}

func remoteNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 404")
}

func (s *AppService) recordSync(kind string) {
	file := syncConfigFile{Provider: "webdav", Filename: "larder-data.zip"}
	if raw, err := os.ReadFile(s.syncConfigPath()); err == nil {
		_ = json.Unmarshal(raw, &file)
	}
	file.LastSyncAt = time.Now().UTC().Format(time.RFC3339)
	file.LastSyncKind = kind
	raw, err := json.MarshalIndent(file, "", "  ")
	if err == nil {
		_ = os.WriteFile(s.syncConfigPath(), raw, 0o600)
	}
}

func (s *AppService) TestSync() error {
	client, err := s.remoteClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(s.context(), 30*time.Second)
	defer cancel()
	return client.Test(ctx)
}

func (s *AppService) UploadSyncPackage(selection transfer.Selection, allowPlaintext, force bool) error {
	client, err := s.remoteClient()
	if err != nil {
		return err
	}
	data, err := s.buildDataPackage(selection)
	if err != nil {
		return err
	}
	config := s.SyncConfig()
	if config.EncryptionEnabled {
		passphrase, err := platform.GetKeychainSecret(encryptionCredentialAccount)
		if err != nil {
			return errors.New("encryption passphrase is not available")
		}
		data, err = remotesync.EncryptPackage(data, passphrase)
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(s.context(), 60*time.Second)
	defer cancel()
	if err := client.Upload(ctx, data); err != nil {
		return err
	}
	s.recordSync("upload")
	return nil
}

func (s *AppService) DownloadSyncPackage() (string, error) {
	client, err := s.remoteClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(s.context(), 60*time.Second)
	defer cancel()
	data, err := client.Download(ctx)
	if err != nil {
		return "", err
	}
	remoteHash := sha256Hex(data)
	config := s.SyncConfig()
	if config.EncryptionEnabled {
		passphrase, err := platform.GetKeychainSecret(encryptionCredentialAccount)
		if err != nil {
			return "", errors.New("encryption passphrase is not available")
		}
		data, err = remotesync.DecryptPackage(data, passphrase)
		if err != nil {
			return "", err
		}
	} else if remotesync.IsEncrypted(data) {
		return "", errors.New("remote package is encrypted; enable encryption first")
	}
	dir := filepath.Join(s.dataDir, "downloads")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "larder-sync-"+time.Now().Format("20060102-150405")+".zip")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	s.recordRemoteHash(remoteHash)
	s.recordSync("download")
	return path, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
