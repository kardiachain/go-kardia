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

package dual

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
)

// EthClient provides read/write functions to data in Ethereum subnode.
// This is implements with a mixture of direct access on the node , or internal RPC calls.
type KardiaEthClient struct {
	ethClient *ethclient.Client
	stack     *node.Node // The running Ethereum node
}

// SyncDetails returns the current sync status of the node.
func (e *KardiaEthClient) NodeSyncStatus() (*ethereum.SyncProgress, error) {
	return e.ethClient.SyncProgress(context.Background())
}
