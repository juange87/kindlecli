// internal/amazon/signer.go
package amazon

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"strings"
	"time"
)

// Signer implements RSA request signing for Amazon's Send-to-Kindle API.
type Signer struct {
	PrivateKey *rsa.PrivateKey
	ADPToken   string
}

// DigestHeaderForRequest computes the X-ADP-Request-Digest header value.
// If signingDate is empty, the current UTC time is used.
func (s *Signer) DigestHeaderForRequest(method, path, postData, signingDate string) string {
	if signingDate == "" {
		signingDate = getSigningDate()
	}

	sigData := strings.Join([]string{method, path, signingDate, postData, s.ADPToken}, "\n")
	hash := sha256.Sum256([]byte(sigData))

	sig := rsaSignCustomPadding(s.PrivateKey, hash[:])

	return base64.StdEncoding.EncodeToString(sig) + ":" + signingDate
}

// rsaSignCustomPadding replicates stkclient's signing scheme:
// PKCS#1 v1.5 type 1 padding without DigestInfo prefix.
// Block: 0x00 || 0x01 || 0xFF * (keySize - hashLen - 3) || 0x00 || hash
// Then raw RSA: c = m^d mod n
func rsaSignCustomPadding(key *rsa.PrivateKey, hash []byte) []byte {
	keySize := key.Size() // bytes (256 for 2048-bit)

	padded := make([]byte, keySize)
	// padded[0] = 0x00 (already zero from make)
	padded[1] = 0x01
	psLen := keySize - len(hash) - 3
	for i := 2; i < 2+psLen; i++ {
		padded[i] = 0xFF
	}
	// padded[2+psLen] = 0x00 (already zero from make)
	copy(padded[keySize-len(hash):], hash)

	m := new(big.Int).SetBytes(padded)
	c := new(big.Int).Exp(m, key.D, key.N)

	result := make([]byte, keySize)
	cBytes := c.Bytes()
	copy(result[keySize-len(cBytes):], cBytes)

	return result
}

func getSigningDate() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
