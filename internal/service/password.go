package service

import (
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890-_"

func GenerateRandomPassword(length int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	pwd := make([]byte, length)
	for i := range pwd {
		pwd[i] = charset[seededRand.Intn(len(charset)-1)]
	}

	return string(pwd)
}
