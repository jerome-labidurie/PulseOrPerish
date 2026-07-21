package fscrypt

import (
	"bytes"
	"testing"
)

func TestPwdToKey_Length(t *testing.T) {
	fc := FsCrypt{Password: "hunter2"}
	salt := make([]byte, saltSize)
	key := fc.pwdToKey(salt)
	if len(key) != int(keySize) {
		t.Errorf("expected key length %d, got %d", keySize, len(key))
	}
}

func TestPwdToKey_Deterministic(t *testing.T) {
	fc := FsCrypt{Password: "same-password"}
	salt := []byte("0123456789abcdef") // 16 bytes

	key1 := fc.pwdToKey(salt)
	key2 := fc.pwdToKey(salt)
	if !bytes.Equal(key1, key2) {
		t.Error("same password+salt should always produce the same key")
	}
}

func TestPwdToKey_DifferentSalts(t *testing.T) {
	fc := FsCrypt{Password: "same-password"}
	salt1 := bytes.Repeat([]byte{0x00}, saltSize)
	salt2 := bytes.Repeat([]byte{0xff}, saltSize)

	key1 := fc.pwdToKey(salt1)
	key2 := fc.pwdToKey(salt2)
	if bytes.Equal(key1, key2) {
		t.Error("different salts should produce different keys")
	}
}

func TestPwdToKey_DifferentPasswords(t *testing.T) {
	salt := bytes.Repeat([]byte{0x42}, saltSize)

	key1 := FsCrypt{Password: "password-a"}.pwdToKey(salt)
	key2 := FsCrypt{Password: "password-b"}.pwdToKey(salt)
	if bytes.Equal(key1, key2) {
		t.Error("different passwords should produce different keys")
	}
}

func TestPwdToKey_EmptyPassword(t *testing.T) {
	fc := FsCrypt{Password: ""}
	salt := bytes.Repeat([]byte{0x01}, saltSize)
	key := fc.pwdToKey(salt)
	if len(key) != int(keySize) {
		t.Errorf("expected key length %d, got %d", keySize, len(key))
	}
}
