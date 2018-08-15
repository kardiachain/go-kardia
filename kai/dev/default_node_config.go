// Defines default configs used for initializing nodes in dev settings.

package dev

import (
	"crypto/ecdsa"
	"fmt"
	"strings"
	"os"
	"encoding/csv"
	"bufio"
	"io"
	"strconv"

	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	con "github.com/kardiachain/go-kardia/consensus/types"

)

type DevNodeConfig struct {
	PrivKey     *ecdsa.PrivateKey
	VotingPower int64
}

type DevEnvironmentConfig struct {
	DevNodeSet []DevNodeConfig

	proposalIndex int
	VotingStrategy []VotingStrategy
}

type node struct {
	key         string
	votingPower int64
}

type VotingStrategy struct {
	Height		*cmn.BigInt
	Round   	*cmn.BigInt
	Step    	con.RoundStepType
	VoteType 	*cmn.BigInt
}

var nodes = []node{
	{"8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06", 100},
	{"77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79", 100},
	{"98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb", 100},
	{"32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe", 100},
	{"68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a", 100},
	{"049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265", 100},
	{"9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007", 100},
	{"ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7", 100},
	{"b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67", 100},
	{"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d", 100},
}

func CreateDevEnvironmentConfig() *DevEnvironmentConfig {
	var devEnv DevEnvironmentConfig
	devEnv.proposalIndex = 0 // Default to 0-th node as the proposer.
	devEnv.DevNodeSet = make([]DevNodeConfig, len(nodes))
	for i, n := range nodes {
		privKey, _ := crypto.ToECDSA([]byte(n.key[:32]))
		devEnv.DevNodeSet[i].PrivKey = privKey
		devEnv.DevNodeSet[i].VotingPower = n.votingPower
	}

	return &devEnv
}


func (devEnv *DevEnvironmentConfig) SetVotingStrategy(votingStrategy string) {
	if strings.HasSuffix(votingStrategy, "csv") {
		csvFile, _ := os.Open(votingStrategy)
		reader := csv.NewReader(bufio.NewReader(csvFile))
		for {
			line, error := reader.Read()
			if error == io.EOF {
				break
			} else if error != nil {
				log.Error("error", error)
			}
			var height, _ = strconv.ParseInt(line[0], 10, 64)
			var round, _ = strconv.ParseInt(line[1],10, 64)
			var step, _ = strconv.ParseInt(line[2],10, 64)
			var voteType, _ = strconv.ParseInt(line[3],10, 64)

			devEnv.VotingStrategy = append(devEnv.VotingStrategy, VotingStrategy{
				Height: 	cmn.NewBigInt(height),
				Round:  	cmn.NewBigInt(round),
				Step: 		con.RoundStepType(step),
				VoteType: 	cmn.NewBigInt(voteType),
			})
		}
	}

}

func (devEnv *DevEnvironmentConfig) SetProposerIndex(index int) {
	if index < 0 || index >= devEnv.GetNodeSize() {
		log.Error(fmt.Sprintf("Proposer index must be within %v and %v", 0, devEnv.GetNodeSize()))
	}
	devEnv.proposalIndex = index
}

func (devEnv *DevEnvironmentConfig) GetDevNodeConfig(index int) *DevNodeConfig {
	return &devEnv.DevNodeSet[index]
}

func (devEnv *DevEnvironmentConfig) GetNodeSize() int {
	return len(devEnv.DevNodeSet)
}

func (devEnv *DevEnvironmentConfig) GetValidatorSet(numVal int) *types.ValidatorSet {
	if numVal < 0 || numVal >= devEnv.GetNodeSize() {
		log.Error(fmt.Sprintf("Number of validator must be within %v and %v", 0, devEnv.GetNodeSize()))
	}
	validators := make([]*types.Validator, numVal)
	for i := 0; i < numVal; i++ {
		node := devEnv.DevNodeSet[i]
		validators[i] = types.NewValidator(node.PrivKey.PublicKey, node.VotingPower)
	}

	validatorSet := types.NewValidatorSet(validators)
	validatorSet.TurnOnKeepSameProposer()
	validatorSet.SetProposer(validators[devEnv.proposalIndex])
	return validatorSet
}
