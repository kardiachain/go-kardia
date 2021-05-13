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

package kvm

import (
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
)

// ExecutionResult includes all output after executing given kvm
// message no matter the execution itself is successful or not.
type ExecutionResult struct {
	UsedGas    uint64 `json:"usedGas"`    // Total used gas but include the refunded gas
	Err        error  `json:"err"`        // Any error encountered during the execution(listed in kvm/errors.go)
	ReturnData []byte `json:"returnData"` // Returned data from kvm (function result or data supplied with revert opcode)
}

// Unwrap returns the internal kvm error which allows us for further
// analysis outside.
func (result *ExecutionResult) Unwrap() error {
	return result.Err
}

// Failed returns the indicator whether the execution is successful or not
func (result *ExecutionResult) Failed() bool { return result.Err != nil }

// Return is a helper function to help caller distinguish between revert reason
// and function return. Return returns the data after execution if no error occurs.
func (result *ExecutionResult) Return() []byte {
	if result.Err != nil {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// Revert returns the concrete revert reason if the execution is aborted by `REVERT`
// opcode. Note the reason can be nil if no data supplied with revert opcode.
func (result *ExecutionResult) Revert() []byte {
	if result.Err != ErrExecutionReverted {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// UnpackRevertReason returns the reason why a KVM call is reverted
func (result *ExecutionResult) UnpackRevertReason() (string, error) {
	if result.Err != ErrExecutionReverted {
		return "", nil
	}
	return abi.UnpackRevert(result.ReturnData)
}
