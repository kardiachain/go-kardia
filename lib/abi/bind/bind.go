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

package bind

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kardiachain/go-kardiamain/lib/abi"
)

// Lang is a target programming language selector to generate bindings for.
type Lang int

const (
	LangGo Lang = iota
	LangJava
	LangObjC
)

// bindBasicTypeGo converts basic solidity types(except array, slice and tuple) to Go one.
func bindBasicTypeGo(kind abi.Type) string {
	switch kind.T {
	case abi.AddressTy:
		return "common.Address"
	case abi.IntTy, abi.UintTy:
		parts := regexp.MustCompile(`(u)?int([0-9]*)`).FindStringSubmatch(kind.String())
		switch parts[2] {
		case "8", "16", "32", "64":
			return fmt.Sprintf("%sint%s", parts[1], parts[2])
		}
		return "*big.Int"
	case abi.FixedBytesTy:
		return fmt.Sprintf("[%d]byte", kind.Size)
	case abi.BytesTy:
		return "[]byte"
	case abi.FunctionTy:
		return "[24]byte"
	default:
		// string, bool types
		return kind.String()
	}
}

// bindTypeGo converts solidity types to Go ones. Since there is no clear mapping
// from all Solidity types to Go ones (e.g. uint17), those that cannot be exactly
// mapped will use an upscaled type (e.g. BigDecimal).
func bindTypeGo(kind abi.Type, structs map[string]*tmplStruct) string {
	switch kind.T {
	case abi.TupleTy:
		return structs[kind.TupleRawName+kind.String()].Name
	case abi.ArrayTy:
		return fmt.Sprintf("[%d]", kind.Size) + bindTypeGo(*kind.Elem, structs)
	case abi.SliceTy:
		return "[]" + bindTypeGo(*kind.Elem, structs)
	default:
		return bindBasicTypeGo(kind)
	}
}

// bindBasicTypeJava converts basic solidity types(except array, slice and tuple) to Java one.
func bindBasicTypeJava(kind abi.Type) string {
	switch kind.T {
	case abi.AddressTy:
		return "Address"
	case abi.IntTy, abi.UintTy:
		// Note that uint and int (without digits) are also matched,
		// these are size 256, and will translate to BigInt (the default).
		parts := regexp.MustCompile(`(u)?int([0-9]*)`).FindStringSubmatch(kind.String())
		if len(parts) != 3 {
			return kind.String()
		}
		// All unsigned integers should be translated to BigInt since gomobile doesn't
		// support them.
		if parts[1] == "u" {
			return "BigInt"
		}

		namedSize := map[string]string{
			"8":  "byte",
			"16": "short",
			"32": "int",
			"64": "long",
		}[parts[2]]

		// default to BigInt
		if namedSize == "" {
			namedSize = "BigInt"
		}
		return namedSize
	case abi.FixedBytesTy, abi.BytesTy:
		return "byte[]"
	case abi.BoolTy:
		return "boolean"
	case abi.StringTy:
		return "String"
	case abi.FunctionTy:
		return "byte[24]"
	default:
		return kind.String()
	}
}

// pluralizeJavaType explicitly converts multidimensional types to predefined
// type in go side.
func pluralizeJavaType(typ string) string {
	switch typ {
	case "boolean":
		return "Bools"
	case "String":
		return "Strings"
	case "Address":
		return "Addresses"
	case "byte[]":
		return "Binaries"
	case "BigInt":
		return "BigInts"
	}
	return typ + "[]"
}

// bindTypeJava converts a Solidity type to a Java one. Since there is no clear mapping
// from all Solidity types to Java ones (e.g. uint17), those that cannot be exactly
// mapped will use an upscaled type (e.g. BigDecimal).
func bindTypeJava(kind abi.Type, structs map[string]*tmplStruct) string {
	switch kind.T {
	case abi.TupleTy:
		return structs[kind.TupleRawName+kind.String()].Name
	case abi.ArrayTy, abi.SliceTy:
		return pluralizeJavaType(bindTypeJava(*kind.Elem, structs))
	default:
		return bindBasicTypeJava(kind)
	}
}

// capitalise makes a camel-case string which starts with an upper case character.
var capitalise = abi.ToCamelCase

// decapitalise makes a camel-case string which starts with a lower case character.
func decapitalise(input string) string {
	if len(input) == 0 {
		return input
	}

	goForm := abi.ToCamelCase(input)
	return strings.ToLower(goForm[:1]) + goForm[1:]
}
