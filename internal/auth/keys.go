package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	PrivateKeyFile = "auth-key.pem"
	PublicKeyFile  = "auth-key.pub.pem"
)

// LoadPrivateKey reads auth-key.pem from dir.
func LoadPrivateKey(dir string) (*ecdsa.PrivateKey, error) {
	path := filepath.Join(dir, PrivateKeyFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	key, err := ParseES256PrivateKey(string(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return key, nil
}

// LoadPublicKey reads auth-key.pub.pem from dir.
func LoadPublicKey(dir string) (*ecdsa.PublicKey, error) {
	path := filepath.Join(dir, PublicKeyFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	key, err := ParseES256PublicKey(string(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return key, nil
}

// ParseES256PublicKey parses a PEM-encoded PKIX P-256 ECDSA public key.
func ParseES256PublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	pemStr = strings.TrimSpace(pemStr)
	if pemStr == "" {
		return nil, fmt.Errorf("public key is required")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("must be a PEM-encoded public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("public key: %w", err)
	}
	ec, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("must be an ECDSA public key")
	}
	if ec.Curve != elliptic.P256() {
		return nil, fmt.Errorf("must use P-256")
	}
	return ec, nil
}
