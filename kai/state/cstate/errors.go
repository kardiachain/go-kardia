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

package cstate

import (
	"errors"
	"fmt"
)

type (
	ErrInvalidBlock      error
	ErrNoValSetForHeight struct {
		Height uint64
	}
	ErrNoConsensusParamsForHeight struct {
		Height uint64
	}
)

func (e ErrNoValSetForHeight) Error() string {
	return fmt.Sprintf("could not find validator set for height #%d", e.Height)
}

func (e ErrNoConsensusParamsForHeight) Error() string {
	return fmt.Sprintf("could not find consensus params for height #%d", e.Height)
}

var (
	ErrNilState = errors.New("nil state")
)
