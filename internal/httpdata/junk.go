package httpdata

import (
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"time"
)

// GenerateJunkParam generates a random string of 3-8 chars for query parameters.
func GenerateJunkParam() string {
	n := rand.Intn(6) + 3
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(rand.Intn(26))
	}
	return string(b)
}

// GenerateJunkValue generates a random string of 1-10 chars for parameter values.
func GenerateJunkValue() string {
	n := rand.Intn(10) + 1
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(rand.Intn(26))
	}
	return string(b)
}

// MD5Sum calculates the MD5 hash of a string.
func MD5Sum(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// GenerateRandomSessionID generates a random session ID hash based on time.
func GenerateRandomSessionID() string {
	return MD5Sum(time.Now().String() + GenerateJunkValue())
}
