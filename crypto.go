package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// GetEncryptionKey забирает ENCRYPTION_KEY из .env и приводит его к 32 байтам
func GetEncryptionKey() ([]byte, error) {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY не установлен в .env")
	}
	key := []byte(keyStr)
	if len(key) >= 32 {
		return key[:32], nil
	}
	padded := make([]byte, 32)
	copy(padded, key)
	return padded, nil
}

// Encrypt шифрует строку plainText с помощью ключа key (должен быть 32 байта)
func Encrypt(plainText string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Создаем случайный nonce (одноразовый код для каждого шифрования)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Шифруем текст и объединяем nonce с зашифрованными данными
	cipherText := gcm.Seal(nonce, nonce, []byte(plainText), nil)

	// Кодируем в base64, чтобы удобно хранить в текстовом поле SQLite
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// Decrypt расшифровывает строку cryptoText
func Decrypt(cryptoText string, key []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()

	nonce, cipherText := data[:nonceSize], data[nonceSize:]
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}

	return string(plainText), nil
}
