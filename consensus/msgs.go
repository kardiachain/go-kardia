package consensus

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	cstypes "github.com/kardiachain/go-kardiamain/consensus/types"
	"github.com/kardiachain/go-kardiamain/lib/common"
	kcons "github.com/kardiachain/go-kardiamain/proto/kardiachain/consensus"
	"github.com/kardiachain/go-kardiamain/types"
)

// MsgToProto takes a consensus message type and returns the proto defined consensus message
func MsgToProto(msg Message) (*kcons.Message, error) {
	if msg == nil {
		return nil, ErrNilMsg
	}

	var pb kcons.Message

	switch msg := msg.(type) {
	case *NewRoundStepMessage:
		pb = kcons.Message{
			Sum: &kcons.Message_NewRoundStep{
				NewRoundStep: &kcons.NewRoundStep{
					Height:                msg.Height,
					Round:                 msg.Round,
					Step:                  uint32(msg.Step),
					SecondsSinceStartTime: msg.SecondsSinceStartTime,
					LastCommitRound:       msg.LastCommitRound,
				},
			},
		}
	case *NewValidBlockMessage:
		pbPartSetHeader := msg.BlockPartsHeader.ToProto()
		pbBits := msg.BlockParts.ToProto()
		pb = kcons.Message{
			Sum: &kcons.Message_NewValidBlock{
				NewValidBlock: &kcons.NewValidBlock{
					Height:             msg.Height,
					Round:              msg.Round,
					BlockPartSetHeader: pbPartSetHeader,
					BlockParts:         pbBits,
					IsCommit:           msg.IsCommit,
				},
			},
		}
	case *ProposalMessage:
		pbP := msg.Proposal.ToProto()
		pb = kcons.Message{
			Sum: &kcons.Message_Proposal{
				Proposal: &kcons.Proposal{
					Proposal: *pbP,
				},
			},
		}
	case *ProposalPOLMessage:
		pbBits := msg.ProposalPOL.ToProto()
		pb = kcons.Message{
			Sum: &kcons.Message_ProposalPol{
				ProposalPol: &kcons.ProposalPOL{
					Height:           msg.Height,
					ProposalPolRound: msg.ProposalPOLRound,
					ProposalPol:      *pbBits,
				},
			},
		}
	case *BlockPartMessage:
		parts, err := msg.Part.ToProto()
		if err != nil {
			return nil, fmt.Errorf("msg to proto error: %w", err)
		}
		pb = kcons.Message{
			Sum: &kcons.Message_BlockPart{
				BlockPart: &kcons.BlockPart{
					Height: msg.Height,
					Round:  msg.Round,
					Part:   *parts,
				},
			},
		}
	case *VoteMessage:
		vote := msg.Vote.ToProto()
		pb = kcons.Message{
			Sum: &kcons.Message_Vote{
				Vote: &kcons.Vote{
					Vote: vote,
				},
			},
		}
	case *HasVoteMessage:
		pb = kcons.Message{
			Sum: &kcons.Message_HasVote{
				HasVote: &kcons.HasVote{
					Height: msg.Height,
					Round:  msg.Round,
					Type:   msg.Type,
					Index:  msg.Index,
				},
			},
		}
	case *VoteSetMaj23Message:
		bi := msg.BlockID.ToProto()
		pb = kcons.Message{
			Sum: &kcons.Message_VoteSetMaj23{
				VoteSetMaj23: &kcons.VoteSetMaj23{
					Height:  msg.Height,
					Round:   msg.Round,
					Type:    msg.Type,
					BlockID: bi,
				},
			},
		}
	case *VoteSetBitsMessage:
		bi := msg.BlockID.ToProto()
		bits := msg.Votes.ToProto()

		vsb := &kcons.Message_VoteSetBits{
			VoteSetBits: &kcons.VoteSetBits{
				Height:  msg.Height,
				Round:   msg.Round,
				Type:    msg.Type,
				BlockID: bi,
			},
		}

		if bits != nil {
			vsb.VoteSetBits.Votes = *bits
		}

		pb = kcons.Message{
			Sum: vsb,
		}

	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}

	return &pb, nil
}

// MsgFromProto takes a consensus proto message and returns the native go type
func MsgFromProto(msg *kcons.Message) (Message, error) {
	if msg == nil {
		return nil, ErrNilMsg
	}
	var pb Message

	switch msg := msg.Sum.(type) {
	case *kcons.Message_NewRoundStep:
		rs := msg.NewRoundStep.Step
		pb = &NewRoundStepMessage{
			Height:                msg.NewRoundStep.Height,
			Round:                 msg.NewRoundStep.Round,
			Step:                  cstypes.RoundStepType(rs),
			SecondsSinceStartTime: msg.NewRoundStep.SecondsSinceStartTime,
			LastCommitRound:       msg.NewRoundStep.LastCommitRound,
		}
	case *kcons.Message_NewValidBlock:
		pbPartSetHeader, err := types.PartSetHeaderFromProto(&msg.NewValidBlock.BlockPartSetHeader)
		if err != nil {
			return nil, fmt.Errorf("parts to proto error: %w", err)
		}

		pbBits := new(common.BitArray)
		pbBits.FromProto(msg.NewValidBlock.BlockParts)

		pb = &NewValidBlockMessage{
			Height:           msg.NewValidBlock.Height,
			Round:            msg.NewValidBlock.Round,
			BlockPartsHeader: *pbPartSetHeader,
			BlockParts:       pbBits,
			IsCommit:         msg.NewValidBlock.IsCommit,
		}
	case *kcons.Message_Proposal:
		pbP, err := types.ProposalFromProto(&msg.Proposal.Proposal)
		if err != nil {
			return nil, fmt.Errorf("proposal msg to proto error: %w", err)
		}

		pb = &ProposalMessage{
			Proposal: pbP,
		}
	case *kcons.Message_ProposalPol:
		pbBits := new(common.BitArray)
		pbBits.FromProto(&msg.ProposalPol.ProposalPol)
		pb = &ProposalPOLMessage{
			Height:           msg.ProposalPol.Height,
			ProposalPOLRound: msg.ProposalPol.ProposalPolRound,
			ProposalPOL:      pbBits,
		}
	case *kcons.Message_BlockPart:
		parts, err := types.PartFromProto(&msg.BlockPart.Part)
		if err != nil {
			return nil, fmt.Errorf("blockpart msg to proto error: %w", err)
		}
		pb = &BlockPartMessage{
			Height: msg.BlockPart.Height,
			Round:  msg.BlockPart.Round,
			Part:   parts,
		}
	case *kcons.Message_Vote:
		vote, err := types.VoteFromProto(msg.Vote.Vote)
		if err != nil {
			return nil, fmt.Errorf("vote msg to proto error: %w", err)
		}

		pb = &VoteMessage{
			Vote: vote,
		}
	case *kcons.Message_HasVote:
		pb = &HasVoteMessage{
			Height: msg.HasVote.Height,
			Round:  msg.HasVote.Round,
			Type:   msg.HasVote.Type,
			Index:  msg.HasVote.Index,
		}
	case *kcons.Message_VoteSetMaj23:
		bi, err := types.BlockIDFromProto(&msg.VoteSetMaj23.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetMaj23 msg to proto error: %w", err)
		}
		pb = &VoteSetMaj23Message{
			Height:  msg.VoteSetMaj23.Height,
			Round:   msg.VoteSetMaj23.Round,
			Type:    msg.VoteSetMaj23.Type,
			BlockID: *bi,
		}
	case *kcons.Message_VoteSetBits:
		bi, err := types.BlockIDFromProto(&msg.VoteSetBits.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetBits msg to proto error: %w", err)
		}
		bits := new(common.BitArray)
		bits.FromProto(&msg.VoteSetBits.Votes)

		pb = &VoteSetBitsMessage{
			Height:  msg.VoteSetBits.Height,
			Round:   msg.VoteSetBits.Round,
			Type:    msg.VoteSetBits.Type,
			BlockID: *bi,
			Votes:   bits,
		}
	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}
	if err := pb.ValidateBasic(); err != nil {
		return nil, err
	}
	return pb, nil
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
