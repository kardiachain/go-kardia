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

package strings

import (
	"fmt"
	"strings"
)

// StringInSlice returns true if a is found the list.
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// SplitAndTrim slices s into all subslices separated by sep and returns a
// slice of the string s with all leading and trailing Unicode code points
// contained in cutset removed. If sep is empty, SplitAndTrim splits after each
// UTF-8 sequence. First part is equivalent to strings.SplitN with a count of
// -1.
func SplitAndTrim(s, sep, cutset string) []string {
	if s == "" {
		return []string{}
	}

	spl := strings.Split(s, sep)
	for i := 0; i < len(spl); i++ {
		spl[i] = strings.Trim(spl[i], cutset)
	}
	return spl
}

// Returns true if s is a non-empty printable non-tab ascii character.
func IsASCIIText(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, b := range []byte(s) {
		if 32 <= b && b <= 126 {
			// good
		} else {
			return false
		}
	}
	return true
}

// NOTE: Assumes that s is ASCII as per IsASCIIText(), otherwise panics.
func ASCIITrim(s string) string {
	r := make([]byte, 0, len(s))
	for _, b := range []byte(s) {
		switch {
		case b == 32:
			continue // skip space
		case 32 < b && b <= 126:
			r = append(r, b)
		default:
			panic(fmt.Sprintf("non-ASCII (non-tab) char 0x%X", b))
		}
	}
	return string(r)
}

// StringSliceEqual checks if string slices a and b are equal
func StringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
