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

import (
	"sync"
)

// helpful to not write everywhere struct{}{}
var keyExists = struct{}{}

// AtomicSet defines a thread safe set data structure.
type AtomicSet struct {
	m map[interface{}]struct{}
	maxSize int64
	l sync.RWMutex // we name it because we don't want to expose it
}

// New creates and initialize a new AtomicSet. It's accept a variable number of
// arguments to populate the initial set. If nothing passed a AtomicSet with zero
// size is created.
func NewAtomicSet(maxSize int64) *AtomicSet {
	s := &AtomicSet{
		maxSize: maxSize,
		m: make(map[interface{}]struct{}),
	}

	return s
}

// Add includes the specified items (one or more) to the set. The underlying
// AtomicSet s is modified. If passed nothing it silently returns.
func (s *AtomicSet) Add(items ...interface{}) {
	if len(items) == 0 {
		return
	}

	s.l.Lock()
	defer s.l.Unlock()

	if s.maxSize > 0 {
		loop:
		for int64(len(items)) > s.maxSize {
			for item := range s.m {
				delete(s.m, item)
				continue loop
			}
		}
	}

	for _, item := range items {
		s.m[item] = keyExists
	}
}

// Remove deletes the specified items from the set.  The underlying AtomicSet s is
// modified. If passed nothing it silently returns.
func (s *AtomicSet) Remove(items ...interface{}) {
	s.l.Lock()
	defer s.l.Unlock()

	if len(items) == 0 || len(s.m) == 0 {
		return
	}
	for _, item := range items {
		delete(s.m, item)
	}
}

// Pop  deletes and return an item from the set. The underlying AtomicSet s is
// modified. If set is empty, nil is returned.
func (s *AtomicSet) Pop() interface{} {
	s.l.RLock()
	for item := range s.m {
		s.l.RUnlock()
		s.l.Lock()
		delete(s.m, item)
		s.l.Unlock()
		return item
	}
	s.l.RUnlock()
	return nil
}

// Has looks for the existence of items passed. It returns false if nothing is
// passed. For multiple items it returns true only if all of  the items exist.
func (s *AtomicSet) Has(items ...interface{}) bool {
	// assume checked for empty item, which not exist
	if len(items) == 0 {
		return false
	}

	s.l.RLock()
	defer s.l.RUnlock()

	has := true
	for _, item := range items {
		if _, has = s.m[item]; !has {
			break
		}
	}
	return has
}

// Size returns the number of items in a set.
func (s *AtomicSet) Size() int {
	s.l.RLock()
	defer s.l.RUnlock()

	l := len(s.m)
	return l
}

// Clear removes all items from the set.
func (s *AtomicSet) Clear() {
	s.l.Lock()
	defer s.l.Unlock()

	s.m = make(map[interface{}]struct{})
}

// IsSubset tests whether t is a subset of s.
func (s *AtomicSet) IsSubset(t *AtomicSet) (subset bool) {
	s.l.RLock()
	defer s.l.RUnlock()

	subset = true

	t.Each(func(item interface{}) bool {
		_, subset = s.m[item]
		return subset
	})

	return
}

// Each traverses the items in the AtomicSet, calling the provided function for each
// set member. Traversal will continue until all items in the AtomicSet have been
// visited, or if the closure returns false.
func (s *AtomicSet) Each(f func(item interface{}) bool) {
	s.l.RLock()
	defer s.l.RUnlock()

	for item := range s.m {
		if !f(item) {
			break
		}
	}
}

// List returns a slice of all items. There is also StringSlice() and
// IntSlice() methods for returning slices of type string or int.
func (s *AtomicSet) List() []interface{} {
	s.l.RLock()
	defer s.l.RUnlock()

	list := make([]interface{}, 0, len(s.m))

	for item := range s.m {
		list = append(list, item)
	}

	return list
}

// Copy returns a new AtomicSet with a copy of s.
func (s *AtomicSet) Copy() *AtomicSet {
	copied := NewAtomicSet(s.maxSize)
	copied.Add(s.List()...)
	return copied
}

// Merge is like Union, however it modifies the current set it's applied on
// with the given t set.
func (s *AtomicSet) Merge(t *AtomicSet) {
	s.l.Lock()
	defer s.l.Unlock()

	t.Each(func(item interface{}) bool {
		s.m[item] = keyExists
		return true
	})
}

func (s *AtomicSet) IsEmpty() bool {
	s.l.RLock()
	defer s.l.RUnlock()

	return len(s.m) == 0
}
