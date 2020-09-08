package types

import (
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
)

// VerifySignature checks that the given public key created signature over hash.
// The public key should be in compressed (33 bytes) or uncompressed (65 bytes) format.
// The signature should have the 64 byte [R || S] format.
func VerifySignature(addr common.Address, hash, signature []byte) bool {
	signPubKey, _ := crypto.SigToPub(hash, signature)
	if signPubKey == nil {
		return false
	}

	// TODO(thientn): Verifying signature shouldn't be this complicated. After
	// cleaning up our crypto package, clean up this as well.
	return addr == crypto.PubkeyToAddress(*signPubKey)

}
