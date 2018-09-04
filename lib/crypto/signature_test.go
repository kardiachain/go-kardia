package crypto

import (
	"bytes"
	"github.com/kardiachain/go-kardia/lib/common"
	"testing"
)

func TestEcrecover(t *testing.T) {
	msg := common.MustDecode("0xce0677bb30baa8cf067c88db9811f4333d131bf8bcf12fe7065d211dce971008")
	msgSignature := common.MustDecode("0x90f27b8b488db00b00606796d2987f6a5f59ae62ea05effe84fef5b8b0e549984a691139ad57a3f0b906637673aa2f63d1f55cb1a69199d4009eea23ceaddc9301")
	publicKey := common.MustDecode("0x04e32df42865e97135acfb65f3bae71bdc86f4d49150ad6a440b6f15878109880a0a2b2667f7e725ceea70c673093bf67663e0312623c8e091b13cf2c0f11ef652")

	key, err := Ecrecover(msg, msgSignature)
	if err != nil {
		t.Fatalf("fails to Ecrecover: %s", err)
	}
	if !bytes.Equal(key, publicKey) {
		t.Errorf("invalida public key: want: %x have: %x", publicKey, key)
	}
}
