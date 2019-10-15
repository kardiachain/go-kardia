package ksml

import (
	"fmt"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types/ref"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"math/big"
	"reflect"
	"strconv"
)

const (
	KARDIA_CALL = "KARDIA_CALL"
	currentTimeStamp = "currentTimeStamp"
	currentBlockHeight = "currentBlockHeight"
	validate = "validate"
	endIf = "endif"
	elif = "elif"
	el = "else"
	ping = "ping"
	addVarFunc = "var"
	ifFunc = "if"
	forEachFunc = "forEach"
	loopIndex = "LOOP_INDEX"
	endForEach = "endForEach"
	splitFunc = "split"
	defineFunc = "defineFunc"
	endDefineFunc = "endDefineFunc"
	callFunc = "call"
	getData = "getData"
	trigger = "trigger"
	publish = "publish"
	compare = "cmp"
	mul = "mul"
	div = "div"
	toInt = "int"
	toFloat = "float"
	exp = "exp"
	format = "format"
	round = "round"

	MaximumGasToCallFunction = uint(5000000)
	intType = "int"
	int8Type = "int8"
	int16Type = "int16"
	int32Type = "int32"
	int64Type = "int64"
	uintType = "uint"
	uint8Type = "uint8"
	uint16Type = "uint16"
	uint32Type = "uint32"
	uint64Type = "uint64"
	bigIntType = "bigInt"
	bigFloatType = "bigFloat"
	float64Type = "float64"
	stringType = "string"
	boolType = "bool"
	listType = "list"
	invalidTypeMsg = "invalid variable, expect %v got %v"

	elMinLength = 8
	builtInSmc = "smc"
	builtInFn = "fn"
	globalMessage = "message"
	globalParams = "params"
	globalContractAddress = "contractAddress"
	globalProxyName = "proxyName"
	prefixSeparator = ":"
	paramsSeparator = ","
	messagePackage = "protocol.EventMessage"

	signalContinue = "SIGNAL_CONTINUE"
	signalStop = "SIGNAL_STOP"                   // stop: do nothing after signal is returned
	signalReturn = "SIGNAL_RETURN"               // return: quit params execution but keep processed params and start another process.
)

type function struct {
	name string
	args []string
	patterns []string
}

var (
	sourceIsEmpty = fmt.Errorf("source is empty")
	invalidExpression = fmt.Errorf("invalid expression")
	invalidMethodFormat = fmt.Errorf("invalid method format")
	abiNotFound = fmt.Errorf("abi is not found")
	methodNotFound = fmt.Errorf("method is not found")
	paramsArgumentsNotMatch = fmt.Errorf("params and arguments are not matched")
	paramValueNotCorrect = fmt.Errorf("param's value is not correct")
	unsupportedType = fmt.Errorf("unsupported type")
	invalidIfParams = fmt.Errorf("not enough arguments for If function")
	invalidIfStatement = fmt.Errorf("invalid if statement")
	incorrectReturnedValueInIFFunc = fmt.Errorf("IF func must returns only 1 bool value")
	invalidSignal = fmt.Errorf("invalid signal")
	stopSignal = fmt.Errorf("signal stop has been applied")
	invalidVariables = fmt.Errorf("invalid variables")
	variableNotFound = fmt.Errorf("variable not found")
	invalidForEachParam = fmt.Errorf("invalid for each param")
	invalidForEachStatement = fmt.Errorf("invalid for each statement")
	notEnoughArgsForSplit = fmt.Errorf("not enough arguments for split function")
	notEnoughArgsForFunc = fmt.Errorf("not enough arguments for create/call Func function")
	invalidSplitArgs = fmt.Errorf("invalid split arguments")
	invalidDefineFunc = fmt.Errorf("invalid define function")

	predefinedPrefix = []string{builtInFn, builtInSmc}
	globalVars = map[string]*expr.Decl{
		globalMessage: decls.NewIdent(globalMessage, decls.NewObjectType(messagePackage), nil),
		globalParams: decls.NewIdent(globalParams, decls.Dyn, nil),
		globalContractAddress: decls.NewIdent(globalContractAddress, decls.String, nil),
		globalProxyName: decls.NewIdent(globalProxyName, decls.String, nil),
	}
	signals = map[string]struct{}{
		signalContinue: {},
		signalReturn: {},
		signalStop: {},
	}
)

var (
	BuiltInFuncMap map[string]BuiltInFunc
	supportedTypes = map[string]func(val interface{}) (interface{}, error){
		intType: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Int {
					return val.(int), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, intType, kind.String())
			}
			v, err := strconv.ParseInt(val.(string), 10, 32)
			if err != nil {
				return nil, err
			}
			return int(v), nil
		},
		int8Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Int8 {
					return val.(int8), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, int8Type, kind.String())
			}
			v, err := strconv.ParseInt(val.(string), 10, 8)
			if err != nil {
				return nil, err
			}
			return int8(v), nil
		},
		int16Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Int16 {
					return val.(int16), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, int16Type, kind.String())
			}
			v, err := strconv.ParseInt(val.(string), 10, 16)
			if err != nil {
				return nil, err
			}
			return int16(v), nil
		},
		int32Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Int32 {
					return val.(int32), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, int32Type, kind.String())
			}
			v, err := strconv.ParseInt(val.(string), 10, 32)
			if err != nil {
				return nil, err
			}
			return int32(v), nil
		},
		int64Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Int64 {
					return val.(int64), nil
				} else if kind.String() == "*big.Int" {
					return val.(*big.Int).Int64(), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, int64Type, kind.String())
			}
			return strconv.ParseInt(val.(string), 10, 64)
		},
		uintType: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Uint {
					return val.(uint), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, uintType, kind.String())
			}
			v, err := strconv.ParseUint(val.(string), 10, 32)
			if err != nil {
				return nil, err
			}
			return uint(v), nil
		},
		uint8Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Uint8 {
					return val.(uint8), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, uint8Type, kind.String())
			}
			v, err := strconv.ParseUint(val.(string), 10, 8)
			if err != nil {
				return nil, err
			}
			return uint8(v), nil
		},
		uint16Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Uint16 {
					return val.(uint16), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, uint16Type, kind.String())
			}
			v, err := strconv.ParseUint(val.(string), 10, 16)
			if err != nil {
				return nil, err
			}
			return uint16(v), nil
		},
		uint32Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Uint32 {
					return val.(uint32), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, uint32Type, kind.String())
			}
			v, err := strconv.ParseUint(val.(string), 10, 32)
			if err != nil {
				return nil, err
			}
			return uint32(v), nil
		},
		uint64Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Uint64 {
					return val.(uint64), nil
				} else if kind == reflect.Int64 { // by default CEL convert number to int64
					return uint64(val.(int64)), nil
				} else if reflect.ValueOf(val).Type().String() == "*big.Int" {
					return val.(*big.Int).Uint64(), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, uint64Type, kind.String())
			}
			return strconv.ParseUint(val.(string), 10, 64)
		},
		bigIntType: func(val interface{}) (interface{}, error) {
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Int64 {
					return big.NewInt(val.(int64)), nil
				} else if kind == reflect.Uint64 {
					return big.NewInt(int64(val.(uint64))), nil
				} else if reflect.ValueOf(val).Type().String() == "*big.Int" {
					return val.(*big.Int), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, bigIntType, reflect.ValueOf(val).Type().String())
			}
			v, _ := big.NewInt(0).SetString(val.(string),10)
			return v, nil
		},
		bigFloatType: func(val interface{}) (interface{}, error) {
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Float64 {
					return big.NewFloat(val.(float64)), nil
				} else if reflect.ValueOf(val).Type().String() == "*big.Float" {
					return val.(*big.Float), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, bigFloatType, kind.String())
			}
			v, _ := big.NewFloat(0).SetString(val.(string))
			return v, nil
		},
		float64Type: func(val interface{}) (interface{}, error){
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				if kind == reflect.Float64 {
					return val.(float64), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, uint64Type, kind.String())
			}
			return strconv.ParseFloat(val.(string), 64)
		},
		stringType: func(val interface{}) (interface{}, error) {
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				return InterfaceToString(val)
			}
			return val.(string), nil
		},
		boolType: func(val interface{}) (interface{}, error) {
			return strconv.ParseBool(val.(string))
		},
		listType: func(val interface{}) (interface{}, error) {
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.Array && kind != reflect.Slice {
				return nil, fmt.Errorf(invalidTypeMsg, listType, kind.String())
			}
			return interfaceToSlice(val)
		},
	}
)

func InterfaceToString(val interface{}) (string, error) {
	v := reflect.ValueOf(val)
	if isType("ref.Val", v) {
		return InterfaceToString(val.(ref.Val).Value())
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

func convertToNative(val reflect.Value) (interface{}, error) {
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
	return "", fmt.Errorf("unsupported value type")
}

func interfaceToSlice(val interface{}) ([]interface{}, error) {
	if reflect.TypeOf(val).Kind() != reflect.Slice && reflect.TypeOf(val).Kind() != reflect.Array {
		return nil, fmt.Errorf("invalid list type, expect slice or array, got %v", reflect.TypeOf(val).Kind().String())
	}
	results := make([]interface{}, 0)
	if reflect.TypeOf(val).Elem().String() == "ref.Val" {
		for _, v := range val.([]ref.Val) {
			results = append(results, v.Value())
		}
		return results, nil
	}

	switch reflect.TypeOf(val).Elem().Kind() {
	case reflect.String:
		for _, v := range val.([]string) {
			results = append(results, v)
		}
	case reflect.Bool:
		for _, v := range val.([]bool) {
			results = append(results, v)
		}
	case reflect.Int:
		for _, v := range val.([]int) {
			results = append(results, v)
		}
	case reflect.Int8:
		for _, v := range val.([]int8) {
			results = append(results, v)
		}
	case reflect.Int16:
		for _, v := range val.([]int16) {
			results = append(results, v)
		}
	case reflect.Int32:
		for _, v := range val.([]int32) {
			results = append(results, v)
		}
	case reflect.Int64:
		for _, v := range val.([]int64) {
			results = append(results, v)
		}
	case reflect.Uint:
		for _, v := range val.([]uint) {
			results = append(results, v)
		}
	case reflect.Uint8:
		for _, v := range val.([]uint8) {
			results = append(results, v)
		}
	case reflect.Uint16:
		for _, v := range val.([]uint16) {
			results = append(results, v)
		}
	case reflect.Uint32:
		for _, v := range val.([]uint32) {
			results = append(results, v)
		}
	case reflect.Uint64:
		for _, v := range val.([]uint64) {
			results = append(results, v)
		}
	case reflect.Uintptr:
		for _, v := range val.([]uintptr) {
			results = append(results, v)
		}
	case reflect.Float32:
		for _, v := range val.([]float32) {
			results = append(results, v)
		}
	case reflect.Float64:
		for _, v := range val.([]float64) {
			results = append(results, v)
		}
	case reflect.Interface:
		return val.([]interface{}), nil
	default:
		return nil, unsupportedType
	}
	return results, nil
}
