package tx_pool

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
	prototx "github.com/kardiachain/go-kardia/proto/kardiachain/txpool"
	"github.com/kardiachain/go-kardia/types"
)

//-----------------------------------------------------------------------------
// Messages

func decodeMsg(bz []byte) (interface{}, error) {
	msg := prototx.Message{}
	err := msg.Unmarshal(bz)
	if err != nil {
		return nil, err
	}

	var message interface{}

	switch msg := msg.Sum.(type) {
	case *prototx.Message_Txs:
		txs := msg.Txs.GetTxs()

		if len(txs) == 0 {
			return message, errors.New("empty TxsMessage")
		}

		decoded := make([]*types.Transaction, len(txs))
		for j, txBytes := range txs {
			tx := &types.Transaction{}
			if err := rlp.DecodeBytes(txBytes, tx); err != nil {
				return message, err
			}

			decoded[j] = tx
		}

		message = TxsMessage{
			Txs: decoded,
		}
	case *prototx.Message_PooledTransactionHashes:
		hashes := msg.PooledTransactionHashes.Hashes
		if len(hashes) == 0 {
			return message, errors.New("empty PooledTransactionHashes")
		}

		decoded := make(NewPooledTransactionHashes, len(hashes))
		for j, hashBytes := range hashes {
			decoded[j] = common.BytesToHash(hashBytes)
		}
		message = decoded
	case *prototx.Message_PooledTransactions:
		txs := msg.PooledTransactions.Txs

		if len(txs) == 0 {
			return message, errors.New("empty TxsMessage")
		}

		pooledTransactions := make(PooledTransactions, len(txs))
		for j, txBytes := range txs {
			tx := &types.Transaction{}
			if err := rlp.DecodeBytes(txBytes, tx); err != nil {
				return message, err
			}

			pooledTransactions[j] = tx
		}
		message = pooledTransactions
	case *prototx.Message_RequestPooledTransactions:
		hashes := msg.RequestPooledTransactions.Hashes
		if len(hashes) == 0 {
			return message, errors.New("empty PooledTransactionHashes")
		}

		decoded := make(RequestPooledTransactionHashes, len(hashes))
		for j, hashBytes := range hashes {
			decoded[j] = common.BytesToHash(hashBytes)
		}
		message = decoded
	default:
		return nil, fmt.Errorf("txpool: message not recognized: %T", msg)
	}

	return message, nil
}

//-------------------------------------

// TxsMessage is a Message containing transactions.
type TxsMessage struct {
	Txs []*types.Transaction
}

// String returns a string representation of the TxsMessage.
func (m *TxsMessage) String() string {
	return fmt.Sprintf("[TxsMessage %v]", m.Txs)
}

// NewPooledTransactionHashes represents a transaction announcement packet.
type NewPooledTransactionHashes []common.Hash

func (m NewPooledTransactionHashes) ValidateBasic() error {
	return nil
}

type RequestPooledTransactionHashes []common.Hash

func (m RequestPooledTransactionHashes) ValidateBasic() error {
	return nil
}

type PooledTransactions []*types.Transaction

func (m PooledTransactions) ValidateBasic() error {
	return nil
}

// MsgToProto takes a consensus message type and returns the proto defined consensus message
func MsgToProto(msg Message) (*prototx.Message, error) {
	var pb prototx.Message
	if msg == nil {
		return nil, ErrNilMsg
	}
	switch msg := msg.(type) {
	case PooledTransactions:
		encoded := make([][]byte, len(msg))
		for idx, tx := range msg {
			txBytes, err := rlp.EncodeToBytes(tx)
			if err != nil {
				return nil, err
			}
			encoded[idx] = txBytes
		}

		pb = prototx.Message{
			Sum: &prototx.Message_PooledTransactions{
				PooledTransactions: &prototx.PooledTransactions{
					Txs: encoded,
				},
			},
		}
	case NewPooledTransactionHashes:
		hashesBytes := make([][]byte, len(msg))
		for idx, tx := range msg {
			txBytes, err := rlp.EncodeToBytes(tx)
			if err != nil {
				return nil, err
			}
			hashesBytes[idx] = txBytes
		}

		pb = prototx.Message{
			Sum: &prototx.Message_PooledTransactionHashes{
				PooledTransactionHashes: &prototx.PooledTransactionHashes{
					Hashes: hashesBytes,
				},
			},
		}
	case RequestPooledTransactionHashes:
		hashesBytes := make([][]byte, len(msg))
		for idx, tx := range msg {
			txBytes, err := rlp.EncodeToBytes(tx)
			if err != nil {
				return nil, err
			}
			hashesBytes[idx] = txBytes
		}

		pb = prototx.Message{
			Sum: &prototx.Message_RequestPooledTransactions{
				RequestPooledTransactions: &prototx.RequestPooledTransactions{
					Hashes: hashesBytes,
				},
			},
		}
	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}

	return &pb, nil
}

type Message interface {
	ValidateBasic() error
}

// MustEncode takes the reactors msg, makes it proto and marshals it
// this mimics `MustMarshalBinaryBare` in that is panics on error
func MustEncode(msg Message) []byte {
	pb, err := MsgToProto(msg)
	if err != nil {
		panic(err)
	}
	enc, err := proto.Marshal(pb)
	if err != nil {
		panic(err)
	}
	return enc
}
