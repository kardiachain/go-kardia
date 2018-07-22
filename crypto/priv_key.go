package crypto

import {
	"crypto/ecdsa"
}

type PrivKey struct {
	key *ecdsa.PrivateKey
}

// Implements PrivKey
type PrivKeyEd25519 [64]byte

func (privKey *PrivKey) Sign(msg []byte) ([] byte, error) {
	return cypto.Sign(msg, privKey.key)
}

