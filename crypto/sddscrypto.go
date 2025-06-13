package sddscrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"os"

	"golang.org/x/crypto/curve25519"
	"github.com/zalando/go-keyring"
	"github.com/mbbgs/constants"
)

//
// ──────────────────────────────────────────────────────────
//   Section: OS Keyring Storage
// ──────────────────────────────────────────────────────────
//

func getHostID() (string, error) {
	return os.Hostname()
}

// SavePrivateKey stores a private key securely in the OS keyring.
func SavePrivateKey(privKey []byte) error {
	host, err := getHostID()
	if err != nil {
		return err
	}
	return keyring.Set(constants.SDDA_APP, host, string(privKey))
}

// LoadPrivateKey retrieves the private key from the OS keyring.
func LoadPrivateKey() ([]byte, error) {
	host, err := getHostID()
	if err != nil {
		return nil, err
	}

	secret, err := keyring.Get(constants.SDDA_APP, host)
	if err != nil {
		return nil, err
	}
	return []byte(secret), nil
}

//
// ──────────────────────────────────────────────────────────
//   Section: Ed25519 Identity / Signing
// ──────────────────────────────────────────────────────────
//


// GenerateX25519KeyPair generates a Curve25519 (X25519) key pair for ECDH.
func GenerateX25519KeyPair() (privateKey, publicKey []byte, err error) {
	privateKey = make([]byte, 32)
	_, err = rand.Read(privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Clamp the private key as per RFC 7748
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	publicKey, err = curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}

	return privateKey, publicKey, nil
}

// GenerateEd25519Key generates an Ed25519 key pair.
func GenerateEd25519Key() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	return pub, priv, err
}

// ExtractPublicKey returns the public key portion from an Ed25519 private key.
func ExtractPublicKey(priv []byte) []byte {
	if len(priv) != ed25519.PrivateKeySize {
		return nil
	}
	return priv[32:] // Last 32 bytes of Ed25519 private key is the public key
}

// SignNonce creates a signature for a nonce using an Ed25519 private key.
func SignNonce(privateKey ed25519.PrivateKey, nonce []byte) []byte {
	return ed25519.Sign(privateKey, nonce)
}

// VerifyNonceSignature checks if a signature is valid for a given nonce and public key.
func VerifyNonceSignature(publicKey ed25519.PublicKey, nonce, signature []byte) bool {
	return ed25519.Verify(publicKey, nonce, signature)
}

//
// ──────────────────────────────────────────────────────────
//   Section: ECDH (X25519) Key Exchange
// ──────────────────────────────────────────────────────────
//

// DeriveECDHKey computes a shared secret using Curve25519 (X25519).
func DeriveECDHKey(privateKey, peerPublicKey []byte) ([]byte, error) {
	if len(privateKey) != 32 || len(peerPublicKey) != 32 {
		return nil, errors.New("invalid key length for X25519 (must be 32 bytes)")
	}
	return curve25519.X25519(privateKey, peerPublicKey)
}

//
// ──────────────────────────────────────────────────────────
//   Section: AES-GCM Encryption
// ──────────────────────────────────────────────────────────
//

// EncryptWithAESGCM encrypts data using AES-GCM with the provided symmetric key.
func EncryptWithAESGCM(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// DecryptWithAESGCM decrypts AES-GCM encrypted data using the given symmetric key and nonce.
func DecryptWithAESGCM(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}