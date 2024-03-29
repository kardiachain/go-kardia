package cmap

import (
	ksync "github.com/kardiachain/go-kardia/lib/sync"
)

// CMap is a goroutine-safe map
type CMap struct {
	m map[string]interface{}
	l ksync.Mutex
}

func NewCMap() *CMap {
	return &CMap{
		m: make(map[string]interface{}),
	}
}

func (cm *CMap) Set(key string, value interface{}) {
	cm.l.Lock()
	cm.m[key] = value
	cm.l.Unlock()
}

func (cm *CMap) Get(key string) interface{} {
	cm.l.Lock()
	val := cm.m[key]
	cm.l.Unlock()
	return val
}

// GetOrSet returns the existing value if present. Othewise, it stores `newValue` and returns it.
func (cm *CMap) GetOrSet(key string, newValue interface{}) (value interface{}, alreadyExists bool) {

	cm.l.Lock()
	defer cm.l.Unlock()

	if v, ok := cm.m[key]; ok {
		return v, true
	}

	cm.m[key] = newValue
	return newValue, false
}

func (cm *CMap) Has(key string) bool {
	cm.l.Lock()
	_, ok := cm.m[key]
	cm.l.Unlock()
	return ok
}

func (cm *CMap) Delete(key string) {
	cm.l.Lock()
	delete(cm.m, key)
	cm.l.Unlock()
}

func (cm *CMap) Size() int {
	cm.l.Lock()
	size := len(cm.m)
	cm.l.Unlock()
	return size
}

func (cm *CMap) Clear() {
	cm.l.Lock()
	cm.m = make(map[string]interface{})
	cm.l.Unlock()
}

func (cm *CMap) Keys() []string {
	cm.l.Lock()

	keys := make([]string, 0, len(cm.m))
	for k := range cm.m {
		keys = append(keys, k)
	}
	cm.l.Unlock()
	return keys
}

func (cm *CMap) Values() []interface{} {
	cm.l.Lock()
	items := make([]interface{}, 0, len(cm.m))
	for _, v := range cm.m {
		items = append(items, v)
	}
	cm.l.Unlock()
	return items
}
