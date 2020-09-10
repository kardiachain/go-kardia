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
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Fraction defined in terms of a numerator divided by a denominator in int64
// format.
type Fraction struct {
	// The portion of the denominator in the faction, e.g. 2 in 2/3.
	Numerator int64 `json:"numerator"`
	// The value by which the numerator is divided, e.g. 3 in 2/3. Must be
	// positive.
	Denominator int64 `json:"denominator"`
}

func (fr Fraction) String() string {
	return fmt.Sprintf("%d/%d", fr.Numerator, fr.Denominator)
}

// ParseFractions takes the string of a fraction as input i.e "2/3" and converts this
// to the equivalent fraction else returns an error. The format of the string must be
// one number followed by a slash (/) and then the other number.
func ParseFraction(f string) (Fraction, error) {
	o := strings.SplitN(f, "/", -1)
	if len(o) != 2 {
		return Fraction{}, errors.New("incorrect formating: should be like \"1/3\"")
	}
	numerator, err := strconv.ParseInt(o[0], 10, 64)
	if err != nil {
		return Fraction{}, fmt.Errorf("incorrect formatting, err: %w", err)
	}

	denominator, err := strconv.ParseInt(o[1], 10, 64)
	if err != nil {
		return Fraction{}, fmt.Errorf("incorrect formatting, err: %w", err)
	}
	return Fraction{Numerator: numerator, Denominator: denominator}, nil
}
