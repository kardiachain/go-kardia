package rawdb

import (
	"github.com/gogo/protobuf/proto"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/common"
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

func WriteConsensusStateHeight(db kaidb.KeyValueWriter, height uint64, state kstate.State) error {
	bz, err := state.Marshal()
	if err != nil {
		return err
	}

	return db.Put(calcConsensusStateKey(height), bz)
}

func DeleteConsensusStateHeight(db kaidb.KeyValueWriter, height uint64) error {
	return db.Delete(calcConsensusStateKey(height))
}

func ReadConsensusParamsInfo(db kaidb.Reader, hash common.Hash) *kstate.ConsensusParamsInfo {
	buf, err := db.Get(calcConsensusParamsInfoKey(hash))
	if err != nil {
		return nil
	}

	cpi := new(kstate.ConsensusParamsInfo)
	err = proto.Unmarshal(buf, cpi)
	if err != nil {
		return nil
	}

	return cpi
}

func WriteConsensusParamsInfo(db kaidb.KeyValueWriter, hash common.Hash, paramsInfo kstate.ConsensusParamsInfo) error {
	bz, err := paramsInfo.Marshal()
	if err != nil {
		return err
	}

	return db.Put(calcConsensusParamsInfoKey(hash), bz)
}

func DeleteConsensusParamsInfo(db kaidb.KeyValueWriter, hash common.Hash) error {
	return db.Delete(calcConsensusParamsInfoKey(hash))
}

func ReadConsensusValidatorsInfo(db kaidb.Reader, hash common.Hash) *kstate.ValidatorsInfo {
	buf, err := db.Get(calcConsensusValidatorsInfoKey(hash))
	if err != nil {
		return nil
	}

	vi := new(kstate.ValidatorsInfo)
	err = proto.Unmarshal(buf, vi)
	if err != nil {
		return nil
	}

	return vi
}

func WriteConsensusValidatorsInfo(db kaidb.KeyValueWriter, hash common.Hash, valInfo kstate.ValidatorsInfo) error {
	bz, err := valInfo.Marshal()
	if err != nil {
		return err
	}

	return db.Put(calcConsensusValidatorsInfoKey(hash), bz)
}

func DeleteConsensusValidatorsInfo(db kaidb.KeyValueWriter, hash common.Hash) error {
	return db.Delete(calcConsensusValidatorsInfoKey(hash))
}
