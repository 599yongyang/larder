package remotesync

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptPackage(t *testing.T) {
	plain := []byte("larder data package")
	encrypted, err := EncryptPackage(plain, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(encrypted) {
		t.Fatal("package should be encrypted")
	}
	if bytes.Equal(encrypted, plain) {
		t.Fatal("package was not encrypted")
	}
	decrypted, err := DecryptPackage(encrypted, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("decrypted = %q", decrypted)
	}
	if _, err := DecryptPackage(encrypted, "wrong password"); err == nil {
		t.Fatal("wrong password should fail")
	}
}
