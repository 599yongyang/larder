package remotesync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

var encryptionMagic = []byte("LARDER-ENC1\n")

const (
	saltSize     = 16
	nonceSize    = 12
	keySize      = 32
	pbkdf2Rounds = 100000
)

// EncryptPackage wraps a sync package in AES-256-GCM with a password-derived
// key. The format is magic || salt || nonce || ciphertext.
func EncryptPackage(plain []byte, password string) ([]byte, error) {
	if len(password) < 8 {
		return nil, errors.New("encryption password must be at least 8 characters")
	}
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := pbkdf2SHA256([]byte(password), salt, pbkdf2Rounds, keySize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(encryptionMagic)+saltSize+nonceSize+len(plain)+gcm.Overhead())
	out = append(out, encryptionMagic...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plain, nil)
	return out, nil
}

func IsEncrypted(data []byte) bool {
	return len(data) >= len(encryptionMagic) && string(data[:len(encryptionMagic)]) == string(encryptionMagic)
}

func DecryptPackage(payload []byte, password string) ([]byte, error) {
	if !IsEncrypted(payload) {
		return nil, errors.New("remote package is not encrypted")
	}
	offset := len(encryptionMagic)
	if len(payload) < offset+saltSize+nonceSize {
		return nil, errors.New("encrypted package is truncated")
	}
	salt := payload[offset : offset+saltSize]
	nonce := payload[offset+saltSize : offset+saltSize+nonceSize]
	ciphertext := payload[offset+saltSize+nonceSize:]
	key := pbkdf2SHA256([]byte(password), salt, pbkdf2Rounds, keySize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("invalid sync passphrase")
	}
	return plain, nil
}

func pbkdf2SHA256(password, salt []byte, iterations, length int) []byte {
	hLen := sha256.Size
	blocks := (length + hLen - 1) / hLen
	output := make([]byte, 0, blocks*hLen)
	for block := 1; block <= blocks; block++ {
		var counter [4]byte
		binary.BigEndian.PutUint32(counter[:], uint32(block))
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write(counter[:])
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		output = append(output, t...)
	}
	return output[:length]
}
