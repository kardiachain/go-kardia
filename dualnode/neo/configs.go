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

package neo

type NeoConfig struct {
	SubmitTxUrl     string
	CheckTxUrl      string
	ReceiverAddress string
}

var DefaultNeoConfig = NeoConfig{
	SubmitTxUrl:     "http://35.240.175.184:5000",
	CheckTxUrl:      "http://35.240.175.184:4000/api/main_net/v1/get_transaction/",
	ReceiverAddress: "AaXPGsJhyRb55r8tREPWWNcaTHq4iiTFAH",
}

