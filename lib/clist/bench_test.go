/*
 *  Copyright 2020 KardiaChain
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

package clist

import "testing"

func BenchmarkDetaching(b *testing.B) {
	lst := New()
	for i := 0; i < b.N+1; i++ {
		lst.PushBack(i)
	}
	start := lst.Front()
	nxt := start.Next()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start.removed = true
		start.DetachNext()
		start.DetachPrev()
		tmp := nxt
		nxt = nxt.Next()
		start = tmp
	}
}

// This is used to benchmark the time of RMutex.
func BenchmarkRemoved(b *testing.B) {
	lst := New()
	for i := 0; i < b.N+1; i++ {
		lst.PushBack(i)
	}
	start := lst.Front()
	nxt := start.Next()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start.Removed()
		tmp := nxt
		nxt = nxt.Next()
		start = tmp
	}
}

func BenchmarkPushBack(b *testing.B) {
	lst := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lst.PushBack(i)
	}
}
