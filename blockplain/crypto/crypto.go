// blockplain/crypto/crypto.go

package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
)

// KeyPair holds a public and private key
type KeyPair struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// NewKeyPair generates a new key pair
func NewKeyPair() (*KeyPair, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	
	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// SaveToFile saves the key pair to files
func (kp *KeyPair) SaveToFile(privateKeyFile, publicKeyFile string) error {
	// Save private key
	privateKeyBytes, err := x509.MarshalECPrivateKey(kp.PrivateKey)
	if err != nil {
		return err
	}
	
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	
	if err := ioutil.WriteFile(privateKeyFile, privateKeyPEM, 0600); err != nil {
		return err
	}
	
	// Save public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(kp.PublicKey)
	if err != nil {
		return err
	}
	
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	
	return ioutil.WriteFile(publicKeyFile, publicKeyPEM, 0644)
}

// LoadFromFile loads a key pair from files
func LoadFromFile(privateKeyFile, publicKeyFile string) (*KeyPair, error) {
	// Load private key
	privateKeyPEM, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, err
	}
	
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing private key")
	}
	
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	
	// Load public key
	publicKeyPEM, err := ioutil.ReadFile(publicKeyFile)
	if err != nil {
		return nil, err
	}
	
	block, _ = pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing public key")
	}
	
	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	
	publicKey, ok := publicKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not an ECDSA public key")
	}
	
	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// Sign creates a digital signature of data
func (kp *KeyPair) Sign(data []byte) ([]byte, error) {
	h := sha256.Sum256(data)
	
	r, s, err := ecdsa.Sign(rand.Reader, kp.PrivateKey, h[:])
	if err != nil {
		return nil, err
	}
	
	// Convert the signature to a byte array
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	
	// Ensure both components are 32 bytes
	rPadded := make([]byte, 32)
	sPadded := make([]byte, 32)
	
	copy(rPadded[32-len(rBytes):], rBytes)
	copy(sPadded[32-len(sBytes):], sBytes)
	
	// Concatenate r and s
	signature := append(rPadded, sPadded...)
	
	return signature, nil
}

// Verify checks a digital signature
func Verify(publicKey *ecdsa.PublicKey, data, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}
	
	h := sha256.Sum256(data)
	
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	
	return ecdsa.Verify(publicKey, h[:], r, s)
}

// GenerateAndSaveKeys generates a new key pair and saves it to the specified directory
func GenerateAndSaveKeys(keyDir string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %v", err)
	}
	
	// Generate key pair
	keyPair, err := NewKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %v", err)
	}
	
	// Save keys to files
	privateKeyPath := fmt.Sprintf("%s/private.pem", keyDir)
	publicKeyPath := fmt.Sprintf("%s/public.pem", keyDir)
	
	if err := keyPair.SaveToFile(privateKeyPath, publicKeyPath); err != nil {
		return fmt.Errorf("failed to save keys: %v", err)
	}
	
	return nil
}