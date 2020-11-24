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

package configs

// These are the multipliers for ether denominations.
// 1 KAI = 10^9 OXY = 10^18 HYDRO
// Example: To get the 'HYDRO' value of an amount in 'OXY', use
//    new(big.Int).Mul(value, big.NewInt(configs.OXY))
const (
	HYDRO = 1
	OXY   = 1e9
	KAI   = 1e18
)
