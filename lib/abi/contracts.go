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

package abi

import (
	"fmt"
	"github.com/google/cel-go/common/types/ref"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

var (
	methodNotFound = fmt.Errorf("method is not found")
	unsupportedType = fmt.Errorf("unsupported type")
)

// GenerateInputStructs creates structs for all methods from theirs inputs
func GenerateInputStruct(smcABI ABI, input []byte) (*Method, interface{}, error) {
	method, err := smcABI.MethodById(input)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range smcABI.Methods {
		if k == method.Name {
			return method, MakeStruct(v.Inputs), nil
		}
	}
	return nil, nil, fmt.Errorf("method not found")
}

// makeStruct makes a struct from abi arguments
func MakeStruct(args Arguments) interface{} {
	var sfs []reflect.StructField
	for i, arg := range args {
		name := arg.Name
		if name == "" {
			name = fmt.Sprintf("name%v", i)
		}
		sf := reflect.StructField{
			Type: arg.Type.Type,
			Name: strings.Title(name),
		}
		if arg.Name != "" {
			sf.Tag = reflect.StructTag(fmt.Sprintf(`abi:"%v"`, arg.Name))
		}
		sfs = append(sfs, sf)
	}
	st := reflect.StructOf(sfs)
	so := reflect.New(st)
	return so.Interface()
}

func ConvertOutputToNative(o reflect.Value, outputs Arguments) ([]interface{}, error) {
	args := make([]interface{}, 0)
	// if o is a primary type, convert it directly
	if o.Kind() != reflect.Interface && o.Kind() != reflect.Ptr {
		v, err := ConvertToNative(o)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	} else { // otherwise, loop it through outputs and add every field into nestedArgs
		for i, _ := range outputs {
			val := o.Elem().Field(i)
			v, err := ConvertToNative(val)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
	}
	return args, nil
}

// GenerateOutputStructs creates structs for all methods from theirs outputs
func GenerateOutputStruct(smcABI ABI, method string, result []byte) (interface{}, error) {
	for k, v := range smcABI.Methods {
		if k == method {
			var obj interface{}
			if len(v.Outputs) == 1 && v.Outputs[0].Name == "" {
				el := v.Outputs[0].Type.Elem
				if el != nil {
					if el.String() == "*big.Int" {
						return big.NewInt(0), nil
					} else if el.String() == "*big.Float" {
						return big.NewFloat(0), nil
					}
				}
				kind := v.Outputs[0].Type.Kind
				switch kind {
				case reflect.String:
					obj = ""
				case reflect.Bool:
					obj = true
				case reflect.Uint, reflect.Uintptr, reflect.Int:
					obj = big.NewInt(0)
				case reflect.Uint8:
					obj = uint8(0)
				case reflect.Uint16:
					obj = uint16(0)
				case reflect.Uint32:
					obj = uint32(0)
				case reflect.Uint64:
					obj = uint64(0)
				case reflect.Int8:
					obj = int8(0)
				case reflect.Int16:
					obj = int16(0)
				case reflect.Int32:
					obj = int32(0)
				case reflect.Int64:
					obj = int64(0)
				default:
					return "", fmt.Errorf("unsupported value type %v", v.Outputs[0].Type.Kind.String())
				}
				if err := smcABI.Unpack(&obj, method, result); err != nil {
					return nil, err
				}
				return obj, nil
			}
			obj = MakeStruct(v.Outputs)
			if err := smcABI.Unpack(obj, method, result); err != nil {
				return nil, err
			}
			return obj, nil
		}
	}
	return nil, methodNotFound
}

func findOutputs(smcABI ABI, method string) Arguments {
	for k, v := range smcABI.Methods {
		if k == method {
			return v.Outputs
		}
	}
	return nil
}

func getInputs(smcABI ABI, method string) *Arguments {
	for k, v := range smcABI.Methods {
		if k == method {
			return &v.Inputs
		}
	}
	return nil
}

func GetMethodAndParams(smcABI ABI, input []byte) (string, []string, error) {
	if len(input) == 0 {
		return "", nil, nil
	}
	args := make([]string, 0)
	method, str, err := GenerateInputStruct(smcABI, input)
	if err != nil || method == nil {
		return "", nil, err
	}

	if len(input[4:])%32 != 0 {
		return "", nil, err
	}

	if err := method.Inputs.Unpack(str, input[4:]); err != nil {
		return "", nil, err
	}
	obj := reflect.ValueOf(str)
	inputs := getInputs(smcABI, method.Name)
	for i, _ := range *inputs {
		v, err := interfaceToString(obj.Elem().Field(i).Interface())
		if err != nil {
			return "", nil, err
		}
		args = append(args, v)
	}
	return method.Name, args, nil
}

func ConvertToNative(val reflect.Value) (interface{}, error) {
	if val.Type().String() == "*big.Int" {
		return val.Interface().(*big.Int), nil
	} else if val.Type().String() == "*big.Float" {
		return val.Interface().(*big.Float), nil
	}
	kind := val.Kind()
	switch kind {
	case reflect.String:
		return val.String(), nil
	case reflect.Bool:
		return val.Bool(), nil
	case reflect.Uint, reflect.Uintptr:
		v, _ := big.NewInt(0).SetString(strconv.FormatUint(val.Uint(), 10), 10)
		return v, nil
	case reflect.Uint8:
		return uint8(val.Uint()), nil
	case reflect.Uint16:
		return uint16(val.Uint()), nil
	case reflect.Uint32:
		return uint32(val.Uint()), nil
	case reflect.Uint64:
		return val.Uint(), nil
	case reflect.Int:
		v, _ := big.NewInt(0).SetString(strconv.FormatInt(val.Int(), 10), 10)
		return v, nil
	case reflect.Int8:
		return int8(val.Int()), nil
	case reflect.Int16:
		return int16(val.Int()), nil
	case reflect.Int32:
		return int32(val.Int()), nil
	case reflect.Int64:
		return val.Int(), nil
	case reflect.Float32, reflect.Float64:
		return val.Float(), nil
	}
	return "", fmt.Errorf("unsupported value type %v", val.Type().String())
}

func interfaceToString(val interface{}) (string, error) {
	v := reflect.ValueOf(val)
	if isType("ref.Val", v) {
		return interfaceToString(val.(ref.Val).Value())
	} else if isType("big.Int", v) {
		return val.(*big.Int).String(), nil
	} else if isType("big.Float", v) {
		return val.(*big.Float).String(), nil
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), []byte("f")[0], 8, 32), nil
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), []byte("f")[0], 8, 64), nil
	case reflect.String:
		return v.String(), nil
	}
	return "", unsupportedType
}

func isType(t string, vals ...reflect.Value) bool {
	for _, val := range vals {
		if !strings.Contains(val.Type().String(), t) {
			return false
		}
	}
	return true
}
