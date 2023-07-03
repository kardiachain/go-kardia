package rawdb

import (
	"github.com/gogo/protobuf/proto"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	kstate "github.com/kardiachain/go-kardia/proto/kardiachain/state"
)

func ReadConsensusStateHeight(db kaidb.Reader, height uint64) *kstate.State {
	buf, err := db.Get(calcConsensusStateKey(height))
	if err != nil {
		return nil
	}

	sp := new(kstate.State)
	err = proto.Unmarshal(buf, sp)
	if err != nil {
		return nil
	}

	return sp
}

func WriteConsensusStateHeight(db kaidb.KeyValueWriter, height uint64, state kstate.State) {
	bz, err := proto.Marshal(&state)
	if err != nil {
		panic(err)
	}

	db.Put(calcConsensusStateKey(height), bz)
}
