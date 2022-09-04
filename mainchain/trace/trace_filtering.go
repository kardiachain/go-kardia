package trace

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/kai/kaidb/bitmapdb"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"

	"github.com/RoaringBitmap/roaring/roaring64"
	jsoniter "github.com/json-iterator/go"
)

// Transaction implements trace_transaction
func (api *TraceAPIImpl) Transaction(ctx context.Context, txHash common.Hash) (ParityTraces, error) {
	tx, err := api.kv.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	chainConfig := api.backend.Config()

	blockHeight, ok := api.backend.TxnLookup(ctx, txHash)
	if !ok {
		return nil, nil
	}

	// Extract transactions from block
	block := api.backend.BlockByHeight(ctx, rpc.BlockHeight(blockHeight))
	if block == nil {
		return nil, fmt.Errorf("could not find block %d", blockHeight)
	}
	var txIndex int
	for idx, txn := range block.Transactions() {
		if txn.Hash() == txHash {
			txIndex = idx
			break
		}
	}
	bn := common.Uint64(blockHeight)

	parentNr := bn
	if parentNr > 0 {
		parentNr -= 1
	}
	hash := block.Hash()

	// Returns an array of trace arrays, one trace array for each transaction
	traces, err := api.callManyTransactions(ctx, tx, block.Transactions(), []string{TraceTypeTrace}, block.LastBlockHash(),
		rpc.BlockHeight(parentNr), block.Header(), txIndex, types.MakeSigner(chainConfig, &blockHeight),
		chainConfig.Rules(new(big.Int).SetUint64(blockHeight)))
	if err != nil {
		return nil, err
	}

	out := make([]ParityTrace, 0, len(traces))
	blockno := uint64(bn)
	for txno, trace := range traces {
		txhash := block.Transactions()[txno].Hash()
		// We're only looking for a specific transaction
		if txno == txIndex {
			for _, pt := range trace.Trace {
				pt.BlockHash = &hash
				pt.BlockNumber = &blockno
				pt.TransactionHash = &txhash
				txpos := uint64(txno)
				pt.TransactionPosition = &txpos
				out = append(out, *pt)
			}
		}
	}

	return out, err
}

// Get implements trace_get
func (api *TraceAPIImpl) Get(ctx context.Context, txHash common.Hash, indicies []common.Uint64) (*ParityTrace, error) {
	// Parity fails if it gets more than a single index. It returns nothing in this case. Must we?
	if len(indicies) > 1 {
		return nil, nil
	}

	traces, err := api.Transaction(ctx, txHash)
	if err != nil {
		return nil, err
	}

	// 'trace_get' index starts at one (oddly)
	firstIndex := int(indicies[0]) + 1
	for i, trace := range traces {
		if i == firstIndex {
			return &trace, nil
		}
	}
	return nil, err
}

// Block implements trace_block
func (api *TraceAPIImpl) Block(ctx context.Context, blockHeight rpc.BlockHeight) (ParityTraces, error) {
	tx, err := api.kv.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Extract transactions from block
	bn := blockHeight.Uint64()
	block := api.backend.BlockByHeight(ctx, blockHeight)
	if block == nil {
		return nil, fmt.Errorf("could not find block %d", bn)
	}
	hash := block.Hash()
	blockInfo := api.backend.BlockInfoByBlockHash(ctx, hash)
	if blockInfo == nil {
		return nil, fmt.Errorf("could not find block info %d", bn)
	}

	parentHeight := bn
	if parentHeight > 0 {
		parentHeight -= 1
	}

	chainConfig := api.backend.Config()
	traces, err := api.callManyTransactions(ctx, tx, block.Transactions(), []string{TraceTypeTrace}, block.LastBlockHash(),
		rpc.BlockHeight(parentHeight), block.Header(), -1, /* all tx indices */
		types.MakeSigner(chainConfig, &bn), chainConfig.Rules(new(big.Int).SetUint64(bn)))
	if err != nil {
		return nil, err
	}

	out := make([]ParityTrace, 0, len(traces))
	blockno := uint64(bn)
	for txno, trace := range traces {
		txhash := block.Transactions()[txno].Hash()
		txpos := uint64(txno)
		for _, pt := range trace.Trace {
			pt.BlockHash = &hash
			pt.BlockNumber = &blockno
			pt.TransactionHash = &txhash
			pt.TransactionPosition = &txpos
			out = append(out, *pt)
		}
	}
	var tr ParityTrace
	var rewardAction = &RewardTraceAction{}
	rewardAction.Author = block.ProposerAddress()
	rewardAction.RewardType = "block" // nolint: goconst
	rewardAction.Value.ToInt().Set(blockInfo.Rewards)
	tr.Action = rewardAction
	tr.BlockHash = &common.Hash{}
	copy(tr.BlockHash[:], block.Hash().Bytes())
	tr.BlockNumber = new(uint64)
	*tr.BlockNumber = block.Height()
	tr.Type = "reward" // nolint: goconst
	tr.TraceAddress = []int{}
	out = append(out, tr)

	return out, err
}

// Filter implements trace_filter
// NOTE: We do not store full traces - we just store index for each address
// Pull blocks which have txs with matching address
func (api *TraceAPIImpl) Filter(ctx context.Context, req TraceFilterRequest, stream *jsoniter.Stream) error {
	dbtx, err1 := api.kv.BeginRo(ctx)
	if err1 != nil {
		return fmt.Errorf("traceFilter cannot open tx: %w", err1)
	}
	defer dbtx.Rollback()

	var fromBlock uint64
	var toBlock uint64
	if req.FromBlock == nil {
		fromBlock = 0
	} else {
		fromBlock = uint64(*req.FromBlock)
	}

	if req.ToBlock == nil {
		headNumber := api.backend.ReadHeaderHeight(ctx, api.backend.ReadHeadBlockHash(ctx))
		toBlock = headNumber
	} else {
		toBlock = uint64(*req.ToBlock)
	}

	if fromBlock > toBlock {
		return fmt.Errorf("invalid parameters: fromBlock cannot be greater than toBlock")
	}

	fromAddresses := make(map[common.Address]struct{}, len(req.FromAddress))
	toAddresses := make(map[common.Address]struct{}, len(req.ToAddress))

	var (
		allBlocks roaring64.Bitmap
		blocksTo  roaring64.Bitmap
	)

	for _, addr := range req.FromAddress {
		if addr != nil {
			b, err := bitmapdb.Get64(dbtx, kvstore.CallFromIndex, addr.Bytes(), fromBlock, toBlock)
			if err != nil {
				if errors.Is(err, kvstore.ErrKeyNotFound) {
					continue
				}
				return err
			}
			allBlocks.Or(b)
			fromAddresses[*addr] = struct{}{}
		}
	}

	for _, addr := range req.ToAddress {
		if addr != nil {
			b, err := bitmapdb.Get64(dbtx, kvstore.CallToIndex, addr.Bytes(), fromBlock, toBlock)
			if err != nil {
				if errors.Is(err, kvstore.ErrKeyNotFound) {
					continue
				}

				return err
			}
			blocksTo.Or(b)
			toAddresses[*addr] = struct{}{}
		}
	}

	switch req.Mode {
	case TraceFilterModeIntersection:
		allBlocks.And(&blocksTo)
	case TraceFilterModeUnion:
		fallthrough
	default:
		allBlocks.Or(&blocksTo)
	}

	// Special case - if no addresses specified, take all traces
	if len(req.FromAddress) == 0 && len(req.ToAddress) == 0 {
		allBlocks.AddRange(fromBlock, toBlock+1)
	} else {
		allBlocks.RemoveRange(0, fromBlock)
		allBlocks.RemoveRange(toBlock+1, uint64(0x100000000))
	}

	chainConfig := api.backend.Config()

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	stream.WriteArrayStart()
	first := true
	// Execute all transactions in picked blocks

	count := uint64(^uint(0)) // this just makes it easier to use below
	if req.Count != nil {
		count = *req.Count
	}
	after := uint64(0) // this just makes it easier to use below
	if req.After != nil {
		after = *req.After
	}
	nSeen := uint64(0)
	nExported := uint64(0)

	it := allBlocks.Iterator()
	for it.HasNext() {
		b := it.Next()
		// Extract transactions from block
		hash := api.backend.ReadCanonicalHash(ctx, b)
		if hash.Equal(common.Hash{}) {
			if first {
				first = false
			} else {
				stream.WriteMore()
			}
			stream.WriteObjectStart()
			stream.WriteObjectEnd()
			continue
		}

		block := api.backend.BlockByHeight(ctx, rpc.BlockHeight(b))
		if block == nil {
			if first {
				first = false
			} else {
				stream.WriteMore()
			}
			stream.WriteObjectStart()
			rpc.HandleError(fmt.Errorf("could not find block %d", b), stream)
			stream.WriteObjectEnd()
			continue
		}
		blockInfo := api.backend.BlockInfoByBlockHash(ctx, hash)
		if blockInfo == nil {
			continue
		}

		blockHash := block.Hash()
		blockNumber := block.Height()
		txs := block.Transactions()
		t, tErr := api.callManyTransactions(ctx, dbtx, txs, []string{TraceTypeTrace}, block.LastBlockHash(),
			rpc.BlockHeight(block.Height()-1), block.Header(), -1, /* all tx indices */
			types.MakeSigner(chainConfig, &b), chainConfig.Rules(new(big.Int).SetUint64(b)))
		if tErr != nil {
			if first {
				first = false
			} else {
				stream.WriteMore()
			}
			stream.WriteObjectStart()
			rpc.HandleError(tErr, stream)
			stream.WriteObjectEnd()
			continue
		}
		includeAll := len(fromAddresses) == 0 && len(toAddresses) == 0
		for i, trace := range t {
			txPosition := uint64(i)
			txHash := txs[i].Hash()
			// Check if transaction concerns any of the addresses we wanted
			for _, pt := range trace.Trace {
				if includeAll || filter_trace(pt, fromAddresses, toAddresses) {
					nSeen++
					pt.BlockHash = &blockHash
					pt.BlockNumber = &blockNumber
					pt.TransactionHash = &txHash
					pt.TransactionPosition = &txPosition
					b, err := json.Marshal(pt)
					if err != nil {
						if first {
							first = false
						} else {
							stream.WriteMore()
						}
						stream.WriteObjectStart()
						rpc.HandleError(err, stream)
						stream.WriteObjectEnd()
						continue
					}
					if nSeen > after && nExported < count {
						if first {
							first = false
						} else {
							stream.WriteMore()
						}
						stream.Write(b)
						nExported++
					}
				}
			}
		}
		if _, ok := toAddresses[block.ProposerAddress()]; ok || includeAll {
			nSeen++
			var tr ParityTrace
			var rewardAction = &RewardTraceAction{}
			rewardAction.Author = block.ProposerAddress()
			rewardAction.RewardType = "block" // nolint: goconst
			rewardAction.Value.ToInt().Set(blockInfo.Rewards)
			tr.Action = rewardAction
			tr.BlockHash = &common.Hash{}
			copy(tr.BlockHash[:], block.Hash().Bytes())
			tr.BlockNumber = new(uint64)
			*tr.BlockNumber = block.Height()
			tr.Type = "reward" // nolint: goconst
			tr.TraceAddress = []int{}
			b, err := json.Marshal(tr)
			if err != nil {
				if first {
					first = false
				} else {
					stream.WriteMore()
				}
				stream.WriteObjectStart()
				rpc.HandleError(err, stream)
				stream.WriteObjectEnd()
				continue
			}
			if nSeen > after && nExported < count {
				if first {
					first = false
				} else {
					stream.WriteMore()
				}
				stream.Write(b)
				nExported++
			}
		}
	}
	stream.WriteArrayEnd()
	return stream.Flush()
}

func filter_trace(pt *ParityTrace, fromAddresses map[common.Address]struct{}, toAddresses map[common.Address]struct{}) bool {
	switch action := pt.Action.(type) {
	case *CallTraceAction:
		_, f := fromAddresses[action.From]
		_, t := toAddresses[action.To]
		if f || t {
			return true
		}
	case *CreateTraceAction:
		_, f := fromAddresses[action.From]
		if f {
			return true
		}

		if res, ok := pt.Result.(*CreateTraceResult); ok {
			if res.Address != nil {
				if _, t := toAddresses[*res.Address]; t {
					return true
				}
			}
		}
	case *SuicideTraceAction:
		_, f := fromAddresses[action.Address]
		_, t := toAddresses[action.RefundAddress]
		if f || t {
			return true
		}
	}

	return false
}

func (api *TraceAPIImpl) callManyTransactions(ctx context.Context, dbtx kvstore.Tx, txs []*types.Transaction,
	traceTypes []string, parentHash common.Hash, parentNo rpc.BlockHeight, header *types.Header, txIndex int,
	signer types.Signer, rules configs.Rules) ([]*TraceCallResult, error) {
	callParams := make([]kaiapi.TransactionArgs, 0, len(txs))
	msgs := make([]types.Message, len(txs))
	for i, tx := range txs {
		hash := tx.Hash()
		callParams = append(callParams, kaiapi.TransactionArgs{
			TxHash:     &hash,
			TraceTypes: traceTypes,
		})
		var err error
		if msgs[i], err = tx.AsMessage(signer); err != nil {
			return nil, fmt.Errorf("convert tx into msg: %w", err)
		}
	}

	traces, cmErr := api.doCallMany(ctx, dbtx, msgs, callParams, &rpc.BlockHeightOrHash{
		BlockHeight:      &parentNo,
		BlockHash:        &parentHash,
		RequireCanonical: true,
	}, header, txIndex)

	if cmErr != nil {
		return nil, cmErr
	}

	return traces, nil
}

// TraceFilterRequest represents the arguments for trace_filter
type TraceFilterRequest struct {
	FromBlock   *common.Uint64    `json:"fromBlock"`
	ToBlock     *common.Uint64    `json:"toBlock"`
	FromAddress []*common.Address `json:"fromAddress"`
	ToAddress   []*common.Address `json:"toAddress"`
	Mode        TraceFilterMode   `json:"mode"`
	After       *uint64           `json:"after"`
	Count       *uint64           `json:"count"`
}

type TraceFilterMode string

const (
	// Default mode for TraceFilter. Unions results referred to addresses from FromAddress or ToAddress
	TraceFilterModeUnion = "union"
	// IntersectionMode retrives results referred to addresses provided both in FromAddress and ToAddress
	TraceFilterModeIntersection = "intersection"
)
