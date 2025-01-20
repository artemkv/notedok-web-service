package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

var NONCE_SIZE = 12
var key = []byte("It was green as an emerald, and the reverberation was stunning.")

func SetEncryptionPassphrase(passphrase string) {
	salt := []byte("champagne and cake")
	key = deriveKey(passphrase, salt)
}

func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, 1000, 32, sha256.New)
}

func encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, NONCE_SIZE)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

func decrypt(ciphertext []byte) ([]byte, error) {
	nonce := ciphertext[:NONCE_SIZE]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext[NONCE_SIZE:], nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
