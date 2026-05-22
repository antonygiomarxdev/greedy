package crypto

import (
	"bytes"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	password := "test-master-password"
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatal(err)
	}

	key := DeriveKey(password, salt)
	plaintext := []byte("SK-ABC123-secret-api-key")

	encrypted, err := Encrypt(plaintext, &key)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := Decrypt(encrypted, &key)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatal("decrypted text does not match plaintext")
	}
}

func TestWrongPassword(t *testing.T) {
	salt, _ := GenerateSalt()
	key := DeriveKey("correct", salt)
	encrypted, _ := Encrypt([]byte("data"), &key)

	wrongKey := DeriveKey("wrong", salt)
	_, err := Decrypt(encrypted, &wrongKey)
	if err == nil {
		t.Fatal("expected error with wrong password")
	}
}

func TestWrongCiphertext(t *testing.T) {
	salt, _ := GenerateSalt()
	key := DeriveKey("pass", salt)
	_, err := Decrypt([]byte("short"), &key)
	if err == nil {
		t.Fatal("expected error with short ciphertext")
	}
}

func TestHashPassword(t *testing.T) {
	salt, _ := GenerateSalt()
	hash := HashPassword("my-pass", salt)

	if !VerifyPassword("my-pass", hash, salt) {
		t.Fatal("password should verify")
	}
	if VerifyPassword("wrong", hash, salt) {
		t.Fatal("wrong password should not verify")
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := []byte("fixed-salt-123456")
	k1 := DeriveKey("pass", salt)
	k2 := DeriveKey("pass", salt)
	if k1 != k2 {
		t.Fatal("key derivation should be deterministic")
	}
}

func TestEncodeDecode(t *testing.T) {
	data := []byte("hello secret data")
	encoded := EncodeB64(data)
	decoded, err := DecodeB64(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, decoded) {
		t.Fatal("base64 roundtrip failed")
	}
}

func TestDecodeInvalid(t *testing.T) {
	_, err := DecodeB64("!!!not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}
