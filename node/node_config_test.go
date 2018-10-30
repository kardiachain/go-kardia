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

package node

import (
	"testing"
)

var nodeIndexTests = []struct {
	input    string
	expected int
}{
	{"", 0},
	{"1", 1},
	{"001232", 1232},
	{"node123", 123},
	{"123node", 0},
	{"123node456", 456},
}

func TestGetNodeIndex(t *testing.T) {
	for _, test := range nodeIndexTests {
		index, _ := GetNodeIndex(test.input)
		if index != test.expected {
			t.Errorf("Expected: %v got: %v", test.expected, index)
		}
	}
}
