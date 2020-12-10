package encoding

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/kardiachain/go-kardia/lib/crypto"
	pc "github.com/kardiachain/go-kardia/proto/kardiachain/crypto"
)

// PubKeyToProto takes crypto.PubKey and transforms it to a protobuf Pubkey
func PubKeyToProto(k ecdsa.PublicKey) (pc.PublicKey, error) {
	var kp pc.PublicKey
	kp = pc.PublicKey{
		Sum: &pc.PublicKey_Ecdsa{
			Ecdsa: crypto.FromECDSAPub(&k),
		},
	}
	return kp, nil
}

// PubKeyFromProto takes a protobuf Pubkey and transforms it to a crypto.Pubkey
func PubKeyFromProto(k pc.PublicKey) (*ecdsa.PublicKey, error) {
	switch k := k.Sum.(type) {
	case *pc.PublicKey_Ecdsa:
		return crypto.UnmarshalPubkey(k.Ecdsa)
	default:
		return nil, fmt.Errorf("fromproto: key type %v is not supported", k)
	}
}
