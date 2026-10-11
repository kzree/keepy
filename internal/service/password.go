package service

import "crypto/rand"

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890-_"

// GenerateRandomPassword returns length characters from the 64-character
// alphabet, or an empty string for nonpositive lengths. crypto/rand.Read fills
// the buffer or terminates the process on entropy failure (Go 1.26 contract).
func GenerateRandomPassword(length int) string {
	if length <= 0 {
		return ""
	}
	pwd := make([]byte, length)
	rand.Read(pwd)
	return passwordFromRandomBytes(pwd)
}

// passwordFromRandomBytes consumes the buffer in place. Modulo is unbiased
// because the fixed alphabet has 64 characters, which divides the 256 byte values.
// Revisit the sampling strategy if the alphabet size changes.
func passwordFromRandomBytes(pwd []byte) string {
	for i, value := range pwd {
		pwd[i] = charset[int(value)%len(charset)]
	}
	return string(pwd)
}
