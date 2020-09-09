/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

// Defines default configs used for initializing nodes in dev settings.

package dev

import (
	"fmt"

	"github.com/kardiachain/go-kardiamain/node"
)

// Nodes are used for testing authorized node in private case
// From 0-9: authorized which are listed in kvm/smc/Permission.sol
// While 10 is not listed mean it is unauthorized.
var Nodes = []map[string]interface{}{
	{
		"key":         "8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06",
		"votingPower": 100,
		"listenAddr":  "[::]:3000",
	},
	{
		"key":         "77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79",
		"votingPower": 100,
		"listenAddr":  "[::]:3001",
	},
	{
		"key":         "98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb",
		"votingPower": 100,
		"listenAddr":  "[::]:3002",
	},
	{
		"key":         "32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe",
		"votingPower": 100,
		"listenAddr":  "[::]:3003",
	},
	{
		"key":         "68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a",
		"votingPower": 100,
		"listenAddr":  "[::]:3004",
	},
	{
		"key":         "049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265",
		"votingPower": 100,
		"listenAddr":  "[::]:3005",
	},
	{
		"key":         "9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007",
		"votingPower": 100,
		"listenAddr":  "[::]:3006",
	},
	{
		"key":         "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7",
		"votingPower": 100,
		"listenAddr":  "[::]:3007",
	},
	{
		"key":         "b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",
		"votingPower": 100,
		"listenAddr":  "[::]:3008",
	},
	{
		"key":         "0cf7ae0332a891044659ace49a0732fa07c2872b4aef479945501f385a23e689",
		"votingPower": 100,
		"listenAddr":  "[::]:3009",
	},
	{
		"key":         "2003be66077b0873f5bedb32a596530eb8a0c908c32dda7771f169ee137c1f82",
		"votingPower": 100,
		"listenAddr":  "[::]:3010",
	},
	{
		"key":         "9dce5ec0b40e363e898f296c01345c12a0961f1cccad098964c73ed85fef5850",
		"votingPower": 100,
		"listenAddr":  "[::]:3011",
	},
	{
		"key":         "f0b2f6f24b70481a51712639badf0e5587545080dc53e0664770adb9881823fb",
		"votingPower": 100,
		"listenAddr":  "[::]:3012",
	},
	{
		"key":         "83731e17afb0da61c0026eaf780364eee367c50a5225ece89a63ad94a4a1f088",
		"votingPower": 100,
		"listenAddr":  "[::]:3013",
	},
	{
		"key":         "fc09d3f004b1ee430fee60568aa29748e277e76f1f372eea9d2b9ff1e27bdfdb",
		"votingPower": 100,
		"listenAddr":  "[::]:3014",
	},
	{
		"key":         "5605dd5f4db003c396956b4b80c093c472ccef4021181aa910125d7c57324152",
		"votingPower": 100,
		"listenAddr":  "[::]:3015",
	},
	// the key below is used for test un-authorized node (private case)
	{
		"key":         "0cf7ae0332a891044659ace49a0732fa07c2872b4aef479945501f385a23e690",
		"votingPower": 0,
		"listenAddr":  "[::]:3016",
	},
}

// GetNodeMetadataByIndex return NodeMetadata from nodes
func GetNodeMetadataByIndex(idx int) (*node.NodeMetadata, error) {
	if idx < 0 || idx >= len(Nodes) {
		return nil, fmt.Errorf("node index must be within 0 to %v", len(Nodes)-1)
	}
	key := Nodes[idx]["key"].(string)
	votingPower := uint64(Nodes[idx]["votingPower"].(int))
	listenAddr := Nodes[idx]["listenAddr"].(string)

	n, err := node.NewNodeMetadata(&key, nil, votingPower, listenAddr)
	if err != nil {
		return nil, err
	}
	return n, nil
}
