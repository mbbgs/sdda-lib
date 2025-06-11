package models

import (
	"runtime"

	"github.com/mbbgs/sdds-lib/utils"
	"github.com/mbbgs/crypto"
)

type Peer struct {
	PeerID             string
	PeerOs             string
	PeerArch           string
	PeerAddr           string
	PeerPort           string
	PeerRole           string
	PeerPubAuthKey     []byte
	
	PeerPubSessionKey  []byte
	PeerPrivSessionKey []byte
}

// NewPeer creates a new Peer instance with generated keys and system metadata.
func NewPeer(peerRole, port string) (*Peer, error) {
	peerID := utils.Generate(11)

	// Generate Ed25519 key pair for authentication
	pubAuthKey, privAuthKey, err := crypto.GenerateEd25519Key()
	if err != nil {
		return nil, utils.WrapError("auth key generation failed", err)
	}

	// Generate X25519 key pair for session encryption
	pubSessKey, privSessKey, err := crypto.GenerateX25519KeyPair()
	if err != nil {
		utils.ZeroBytes(privAuthKey) // Clear the previous key
		return nil, utils.WrapError("session key generation failed", err)
	}

	// Save authentication private key to keyring
	if err := crypto.SavePrivateKeyToKeyring(peerID, privAuthKey); err != nil {
		utils.ZeroBytes(privAuthKey)
		utils.ZeroBytes(privSessKey)
		return nil, utils.WrapError("saving private auth key failed", err)
	}

	// Wipe sensitive keys from memory after use
	utils.ZeroBytes(privAuthKey)

	localIP := getLocalIP()
	if localIP == "" {
		utils.ZeroBytes(privSessKey)
		return nil, utils.Error("could not determine local IP address")
	}

	// Return peer instance
	return &Peer{
		PeerID:             peerID,
		PeerOs:             runtime.GOOS,
		PeerArch:           runtime.GOARCH,
		PeerAddr:           localIP,
		PeerPort:           port,
		PeerRole:           peerRole,
		PeerPubAuthKey:     pubAuthKey,
		PeerPubSessionKey:  pubSessKey,
		PeerPrivSessionKey: privSessKey,
	}, nil
}