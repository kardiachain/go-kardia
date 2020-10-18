package types

import (
	"errors"
	"fmt"

	kaiproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

func (bm *BlockMeta) ToProto() *kaiproto.BlockMeta {
	if bm == nil {
		return nil
	}

	pb := &kaiproto.BlockMeta{
		BlockID: bm.BlockID.ToProto(),
		Header:  *bm.Header.ToProto(),
	}
	return pb
}

func BlockMetaFromProto(pb *kaiproto.BlockMeta) (*BlockMeta, error) {
	if pb == nil {
		return nil, errors.New("blockmeta is empty")
	}

	bm := new(BlockMeta)

	bi, err := BlockIDFromProto(&pb.BlockID)
	if err != nil {
		return nil, err
	}

	h, err := HeaderFromProto(&pb.Header)
	if err != nil {
		return nil, err
	}

	bm.BlockID = *bi
	bm.Header = &h

	return bm, bm.ValidateBasic()
}

// ValidateBasic performs basic validation.
func (bm *BlockMeta) ValidateBasic() error {
	if err := bm.BlockID.ValidateBasic(); err != nil {
		return err
	}
	if !bm.BlockID.Hash.Equal(bm.Header.Hash()) {
		return fmt.Errorf("expected BlockID#Hash and Header#Hash to be the same, got %X != %X",
			bm.BlockID.Hash, bm.Header.Hash())
	}
	return nil
}
