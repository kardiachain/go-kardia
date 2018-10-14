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

package common

import "sync"

// CMap is a goroutine-safe map
type CMap struct {
	m map[string]interface{}
	l sync.Mutex
}

func NewCMap() *CMap {
	return &CMap{
		m: make(map[string]interface{}),
	}
}

func (cm *CMap) Set(key string, value interface{}) {
	cm.l.Lock()
	defer cm.l.Unlock()
	cm.m[key] = value
}

func (cm *CMap) Get(key string) interface{} {
	cm.l.Lock()
	defer cm.l.Unlock()
	return cm.m[key]
}

func (cm *CMap) Has(key string) bool {
	cm.l.Lock()
	defer cm.l.Unlock()
	_, ok := cm.m[key]
	return ok
}

func (cm *CMap) Delete(key string) {
	cm.l.Lock()
	defer cm.l.Unlock()
	delete(cm.m, key)
}

func (cm *CMap) Size() int {
	cm.l.Lock()
	defer cm.l.Unlock()
	return len(cm.m)
}

func (cm *CMap) Clear() {
	cm.l.Lock()
	defer cm.l.Unlock()
	cm.m = make(map[string]interface{})
}

func (cm *CMap) Keys() []string {
	cm.l.Lock()
	defer cm.l.Unlock()

	keys := []string{}
	for k := range cm.m {
		keys = append(keys, k)
	}
	return keys
}

func (cm *CMap) Values() []interface{} {
	cm.l.Lock()
	defer cm.l.Unlock()
	items := []interface{}{}
	for _, v := range cm.m {
		items = append(items, v)
	}
	return items
}
