package crypto

import (
	"crypto/ecdsa"
)

type PrivKey struct {
	key *ecdsa.PrivateKey
}

func (privKey *PrivKey) Sign(msg []byte) ([]byte, error) {
	return Sign(msg, privKey.key)
}
