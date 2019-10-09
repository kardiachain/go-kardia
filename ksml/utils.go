package ksml

import (
	"fmt"
	"github.com/google/cel-go/checker/decls"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"math/big"
	"reflect"
	"strconv"
)

const (
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

	MaximumGasToCallStaticFunction = uint(4000000)
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
	stringType = "string"
	boolType = "bool"
	listType = "list"
	invalidTypeMsg = "invalid variable, expect %v got %v"

	elMinLength = 8
	builtInSmc = "smc"
	builtInFn = "fn"
	globalMessage = "message"
	globalParams = "params"
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
				}else if kind.String() == "*big.Int" {
					return val.(*big.Int), nil
				}
				return nil, fmt.Errorf(invalidTypeMsg, bigIntType, kind.String())
			}
			v, _ := big.NewInt(0).SetString(val.(string),10)
			return v, nil
		},
		stringType: func(val interface{}) (interface{}, error) {
			kind := reflect.TypeOf(val).Kind()
			if kind != reflect.String {
				return nil, fmt.Errorf(invalidTypeMsg, stringType, kind.String())
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
			return val.([]interface{}), nil
		},
	}
)
