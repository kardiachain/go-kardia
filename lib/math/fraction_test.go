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

package math

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFraction(t *testing.T) {

	testCases := []struct {
		f   string
		exp Fraction
		err bool
	}{
		{
			f:   "2/3",
			exp: Fraction{2, 3},
			err: false,
		},
		{
			f:   "15/5",
			exp: Fraction{15, 5},
			err: false,
		},
		{
			f:   "-1/2",
			exp: Fraction{-1, 2},
			err: false,
		},
		{
			f:   "1/-2",
			exp: Fraction{1, -2},
			err: false,
		},
		{
			f:   "2/3/4",
			exp: Fraction{},
			err: true,
		},
		{
			f:   "123",
			exp: Fraction{},
			err: true,
		},
		{
			f:   "1a2/4",
			exp: Fraction{},
			err: true,
		},
		{
			f:   "1/3bc4",
			exp: Fraction{},
			err: true,
		},
	}

	for idx, tc := range testCases {
		output, err := ParseFraction(tc.f)
		if tc.err {
			assert.Error(t, err, idx)
		} else {
			assert.NoError(t, err, idx)
		}
		assert.Equal(t, tc.exp, output, idx)
	}

}
