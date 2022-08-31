/*
Copyright 2021 Erigon contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package kvcache

import (
	"context"

	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/proto/kardiachain/remote"
)

// DummyCache - doesn't remember anything - can be used when service is not remote
type DummyCache struct{}

var _ Cache = (*DummyCache)(nil)    // compile-time interface check
var _ CacheView = (*DummyView)(nil) // compile-time interface check

func NewDummy() *DummyCache { return &DummyCache{} }
func (c *DummyCache) View(_ context.Context, tx kvstore.Tx) (CacheView, error) {
	return &DummyView{cache: c, tx: tx}, nil
}
func (c *DummyCache) OnNewBlock(sc *remote.StateChangeBatch) {}
func (c *DummyCache) Evict() int                             { return 0 }
func (c *DummyCache) Len() int                               { return 0 }
func (c *DummyCache) Get(k []byte, tx kvstore.Tx, id ViewID) ([]byte, error) {
	return tx.GetOne(kvstore.PlainState, k)
}
func (c *DummyCache) GetCode(k []byte, tx kvstore.Tx, id ViewID) ([]byte, error) {
	return tx.GetOne(kvstore.Code, k)
}

type DummyView struct {
	cache *DummyCache
	tx    kvstore.Tx
}

func (c *DummyView) Get(k []byte) ([]byte, error)     { return c.cache.Get(k, c.tx, 0) }
func (c *DummyView) GetCode(k []byte) ([]byte, error) { return c.cache.GetCode(k, c.tx, 0) }
