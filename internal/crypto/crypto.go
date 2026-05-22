package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	SaltSize   = 16
	NonceSize  = 24
	KeySize    = 32
	argonTime  = 1
	argonMem   = 64 * 1024
	argonThreads = 4
)

func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

func DeriveKey(password string, salt []byte) [KeySize]byte {
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMem, argonThreads, KeySize)
	var k [KeySize]byte
	copy(k[:], key)
	return k
}

func Encrypt(plaintext []byte, key *[KeySize]byte) ([]byte, error) {
	var nonce [NonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	// Prepend nonce to ciphertext
	encrypted := secretbox.Seal(nonce[:], plaintext, &nonce, key)
	return encrypted, nil
}

func Decrypt(encrypted []byte, key *[KeySize]byte) ([]byte, error) {
	if len(encrypted) < NonceSize+secretbox.Overhead {
		return nil, fmt.Errorf("ciphertext too short: %d bytes", len(encrypted))
	}
	var nonce [NonceSize]byte
	copy(nonce[:], encrypted[:NonceSize])
	ciphertext := encrypted[NonceSize:]

	decrypted, ok := secretbox.Open(nil, ciphertext, &nonce, key)
	if !ok {
		return nil, fmt.Errorf("decryption failed: wrong password or corrupted data")
	}
	return decrypted, nil
}

func HashPassword(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMem, argonThreads, 32)
}

func VerifyPassword(password string, hash, salt []byte) bool {
	computed := HashPassword(password, salt)
	return string(computed) == string(hash)
}

func EncodeB64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func DecodeB64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
