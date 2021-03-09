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

package state

import (
	"encoding/json"
	"fmt"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/trie"
)

// DumpCollector interface which the state trie calls during iteration
type DumpCollector interface {
	// OnRoot is called with the state root
	OnRoot(common.Hash)
	// OnAccount is called once for each account in the trie
	OnAccount(common.Address, DumpAccount)
}

type DumpAccount struct {
	Balance   string                 `json:"balance"`
	Nonce     uint64                 `json:"nonce"`
	Root      string                 `json:"root"`
	CodeHash  string                 `json:"codeHash"`
	Code      string                 `json:"code"`
	Storage   map[common.Hash]string `json:"storage"`
	Address   *common.Address        `json:"address,omitempty"` // Address only present in iterative (line-by-line) mode
	SecureKey common.Bytes           `json:"key,omitempty"`     // If we don't have address, we can output the key
}

type Dump struct {
	Root     string                         `json:"root"`
	Accounts map[common.Address]DumpAccount `json:"accounts"`
}

// OnRoot implements DumpCollector interface
func (d *Dump) OnRoot(root common.Hash) {
	d.Root = fmt.Sprintf("%x", root)
}

// OnAccount implements DumpCollector interface
func (d *Dump) OnAccount(addr common.Address, account DumpAccount) {
	d.Accounts[addr] = account
}

// IteratorDump is an implementation for iterating over data.
type IteratorDump struct {
	Root     string                         `json:"root"`
	Accounts map[common.Address]DumpAccount `json:"accounts"`
	Next     []byte                         `json:"next,omitempty"` // nil if no more accounts
}

// OnRoot implements DumpCollector interface
func (d *IteratorDump) OnRoot(root common.Hash) {
	d.Root = fmt.Sprintf("%x", root)
}

// OnAccount implements DumpCollector interface
func (d *IteratorDump) OnAccount(addr common.Address, account DumpAccount) {
	d.Accounts[addr] = account
}

// iterativeDump is a DumpCollector-implementation which dumps output line-by-line iteratively.
type iterativeDump struct {
	*json.Encoder
}

// OnRoot implements DumpCollector interface
func (d iterativeDump) OnRoot(root common.Hash) {
	d.Encode(struct {
		Root common.Hash `json:"root"`
	}{root})
}

// OnAccount implements DumpCollector interface
func (d iterativeDump) OnAccount(addr common.Address, account DumpAccount) {
	dumpAccount := &DumpAccount{
		Balance:   account.Balance,
		Nonce:     account.Nonce,
		Root:      account.Root,
		CodeHash:  account.CodeHash,
		Code:      account.Code,
		Storage:   account.Storage,
		SecureKey: account.SecureKey,
	}
	if addr != (common.Address{}) {
		dumpAccount.Address = &addr
	}
	d.Encode(dumpAccount)
}

func (sdb *StateDB) DumpToCollector(c DumpCollector, excludeCode, excludeStorage, excludeMissingPreimages bool, start []byte, maxResults int) (nextKey []byte) {
	missingPreimages := 0
	c.OnRoot(sdb.trie.Hash())

	var count int
	it := trie.NewIterator(sdb.trie.NodeIterator(start))
	for it.Next() {
		var data Account
		if err := rlp.DecodeBytes(it.Value, &data); err != nil {
			panic(err)
		}
		account := DumpAccount{
			Balance:  data.Balance.String(),
			Nonce:    data.Nonce,
			Root:     common.Bytes2Hex(data.Root[:]),
			CodeHash: common.Bytes2Hex(data.CodeHash),
		}
		addrBytes := sdb.trie.GetKey(it.Key)
		if addrBytes == nil {
			// Preimage missing
			missingPreimages++
			if excludeMissingPreimages {
				continue
			}
		}
		addr := common.BytesToAddress(addrBytes)
		obj := newObject(sdb, addr, data)
		if !excludeCode {
			account.Code = common.Bytes2Hex(obj.Code(sdb.db))
		}
		if !excludeStorage {
			account.Storage = make(map[common.Hash]string)
			storageIt := trie.NewIterator(obj.getTrie(sdb.db).NodeIterator(nil))
			for storageIt.Next() {
				_, content, _, err := rlp.Split(storageIt.Value)
				if err != nil {
					log.Error("Failed to decode the value returned by iterator", "error", err)
					continue
				}
				account.Storage[common.BytesToHash(sdb.trie.GetKey(storageIt.Key))] = common.Bytes2Hex(content)
			}
		}
		c.OnAccount(addr, account)
		count++
		if maxResults > 0 && count >= maxResults {
			if it.Next() {
				nextKey = it.Key
			}
			break
		}
	}
	if missingPreimages > 0 {
		log.Warn("Dump incomplete due to missing preimages", "missing", missingPreimages)
	}

	return nextKey
}

// RawDump returns the entire state an a single large object
func (sdb *StateDB) RawDump(excludeCode, excludeStorage, excludeMissingPreimages bool) Dump {
	dump := &Dump{
		Accounts: make(map[common.Address]DumpAccount),
	}
	sdb.DumpToCollector(dump, excludeCode, excludeStorage, excludeMissingPreimages, nil, 0)
	return *dump
}

// Dump returns a JSON string representing the entire state as a single json-object
func (sdb *StateDB) Dump(excludeCode, excludeStorage, excludeMissingPreimages bool) []byte {
	dump := sdb.RawDump(excludeCode, excludeStorage, excludeMissingPreimages)
	json, err := json.MarshalIndent(dump, "", "    ")
	if err != nil {
		fmt.Println("Dump err", err)
	}
	return json
}

// IterativeDump dumps out accounts as json-objects, delimited by linebreaks on stdout
func (sdb *StateDB) IterativeDump(excludeCode, excludeStorage, excludeMissingPreimages bool, output *json.Encoder) {
	sdb.DumpToCollector(iterativeDump{output}, excludeCode, excludeStorage, excludeMissingPreimages, nil, 0)
}

// IteratorDump dumps out a batch of accounts starts with the given start key
func (sdb *StateDB) IteratorDump(excludeCode, excludeStorage, excludeMissingPreimages bool, start []byte, maxResults int) IteratorDump {
	iterator := &IteratorDump{
		Accounts: make(map[common.Address]DumpAccount),
	}
	iterator.Next = sdb.DumpToCollector(iterator, excludeCode, excludeStorage, excludeMissingPreimages, start, maxResults)
	return *iterator
}
