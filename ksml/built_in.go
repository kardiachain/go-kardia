package ksml

import (
	"fmt"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	kaiType "github.com/kardiachain/go-kardia/types"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// BuiltInFunc defines common function that is used in BuiltInFuncMap.
// BuiltInFunc is used when `fn` defined in ${...} format in ParseParams function.
// eg: "${fn:currentTimeStamp}"
type BuiltInFunc func(p *Parser, extras ...interface{}) ([]interface{}, error)

func init() {
	BuiltInFuncMap = map[string]BuiltInFunc{
		ping: pong, // this map is used for testing purpose.
		currentTimeStamp: getCurrentTimeStamp,
		currentBlockHeight: getCurrentBlockHeight,
		validate: validateFunc,
		ifFunc: executeIf,
		endIf: emptyFunc,
		elif: emptyFunc,
		el: emptyFunc,
		endForEach: emptyFunc,
		addVarFunc: addVar,
		forEachFunc: forEach,
		splitFunc: split,
	}
}

func emptyFunc(p *Parser, extras ...interface{}) ([]interface{}, error) {
	return nil, fmt.Errorf("this function cannot be called")
}

func getCurrentTimeStamp(p *Parser, extras ...interface{}) ([]interface{}, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	return []interface{}{now}, nil
}

func getCurrentBlockHeight(p *Parser, extras ...interface{}) ([]interface{}, error) {
	height := p.bc.CurrentBlock().Height()
	return []interface{}{int64(height)}, nil
}

func pong(p *Parser, extras ...interface{}) ([]interface{}, error) {
	return []interface{}{"pong"}, nil
}

// addVar adds a variable into parser.userDefinedVariables. extras must has len=3 which [0] is varName, [1] is varType, [2] is value
func addVar(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 3 {
		return nil, invalidVariables
	}
	varName, varType, varVal := extras[0].(string), extras[1].(string), extras[2].(string)
	// apply CEL to varVal
	val, err := p.handleContent(varVal)
	if err != nil {
		return nil, err
	}
	if len(val) == 0 {
		return nil, fmt.Errorf("returned value is empty")
	}
	convertFunc, ok := supportedTypes[varType]
	if !ok {
		return nil, variableNotFound
	}
	v, err := convertFunc(val[0])
	if err != nil {
		return nil, err
	}
	p.userDefinedVariables[varName] = v
	return nil, nil
}

// validateFunc has 3 elements, condition, true signal and false signal.
// if condition is true then true signal is returned otherwise false signal is returned
func validateFunc(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 3 {
		return nil, invalidIfParams
	}
	ifSrc, trueSignal, falseSignal := extras[0].(string), extras[1].(string), extras[2].(string)
	// check if signal is valid or not
	if _, ok := signals[trueSignal]; !ok {
		return nil, invalidSignal
	}
	if _, ok := signals[falseSignal]; !ok {
		return nil, invalidSignal
	}
	// apply CEL to ifSrc. return data must be bool. otherwise return error
	ifResult, err := p.handleContent(ifSrc)
	if err != nil {
		return nil, err
	}
	if len(ifResult) != 1 || reflect.TypeOf(ifResult[0]).Kind() != reflect.Bool {
		return nil, incorrectReturnedValueInIFFunc
	}
	if ifResult[0].(bool) {
		return []interface{}{trueSignal}, nil
	} else {
		return []interface{}{falseSignal}, nil
	}
}

// executeIf executes if blocks. an if structures is start with fn:if(block_name, cond1)...fn:elif(block_name, cond2)...fn:else(block_name)...fn:endif(block_name)
func executeIf(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 2 {
		return nil, invalidIfParams
	}
	// name is used to specify name of ifElse block. name must be unique
	name, condition := extras[0].(string), extras[1].(string)

	// get start position and find end position with format fn:endif(name) and same condition to get a block code
	currentPos := p.pc
	patternBlocks := make(map[string][]string)
	newPatterns := make([]string, 0)
	key := condition
	listCond := make([]string, 0)
	validIfStatement := false

	for _, pattern := range p.globalPatterns[currentPos+1:] {
		if strings.Contains(pattern, name) && (strings.Contains(pattern, endIf)) ||
			strings.Contains(pattern, elif) || strings.Contains(pattern, el) {
			patternBlocks[key] = newPatterns
			listCond = append(listCond, key)
			_, method, results, err := p.getPrefix(strings.ReplaceAll(strings.ReplaceAll(pattern, "}", ""), "${", ""))
			if err != nil {
				return nil, err
			}
			if method == el {
				key = fmt.Sprintf("%v(%v)", el, name)
			} else if method == endIf {
				// move program counter to the next position then break
				p.pc++
				validIfStatement = true
				break
			} else {
				key = results[1]
			}
			// reset newPatterns to prepare for next condition's patterns
			newPatterns = make([]string, 0)
		} else {
			newPatterns = append(newPatterns, pattern)
		}
		p.pc++
	}

	if !validIfStatement { // cannot find endIf
		return nil, invalidIfStatement
	}

	for _, cond := range listCond {
		// if cond is el
		if strings.Contains(cond, el) {
			return parseBlockPatterns(p, patternBlocks[cond], nil)
		} else {
			val, err := p.handleContent(cond)
			if err != nil {
				return nil, err
			}
			if len(val) != 1 || reflect.TypeOf(val[0]).Kind() != reflect.Bool {
				return nil, incorrectReturnedValueInIFFunc
			}
			if val[0].(bool) {
				return parseBlockPatterns(p, patternBlocks[cond], nil)
			}
		}
	}
	return nil, nil
}

// parseBlockPatterns reads nested patterns with different parser then returns all returned params.
func parseBlockPatterns(p *Parser, patterns []string, extrasVar map[string]interface{}) ([]interface{}, error) {
	newParser := NewParser(p.bc, p.stateDb, p.smartContractAddress, patterns, p.globalMessage)
	// add all definedVariables in p in overwrite cases.
	for k, v := range p.userDefinedVariables {
		newParser.userDefinedVariables[k] = v
	}

	if extrasVar != nil {
		for k, v := range extrasVar {
			newParser.userDefinedVariables[k] = v
		}
	}

	err := newParser.ParseParams()
	if err != nil {
		return nil, err
	}
	// update updated variables in newParser
	for k, v := range newParser.userDefinedVariables {
		if _, ok := p.userDefinedVariables[k]; ok {
			p.userDefinedVariables[k] = v
		}
	}
	return newParser.globalParams.([]interface{}), nil
}

// forEach loops through a given list variables and execute all logics inside forEach(name, vars)...endForEach(name) pair.
func forEach(p *Parser, extras ...interface{}) ([]interface{}, error) {
	// extras must have 2 elements: first element is the name of for loop which is used to find forEachEnd.
	// second element must be an array or a slice.
	if len(extras) != 2 {
		return nil, invalidForEachParam
	}

	val, err := p.handleContent(extras[1].(string))
	if err != nil {
		return nil, err
	}

	if val == nil || len(val) == 0 {
		return nil, invalidForEachParam
	}

	if reflect.TypeOf(val[0]).Kind() != reflect.Array && reflect.TypeOf(val[0]).Kind() != reflect.Slice {
		return nil, invalidForEachParam
	}

	name := extras[0].(string)
	newPatterns := make([]string, 0)
	validForEach := false
	// loop globalPatterns from current position until we find
	for _, pattern := range p.globalPatterns[p.pc+1:] {
		if strings.Contains(pattern, name) && strings.Contains(pattern, endForEach) {
			validForEach = true
		} else {
			newPatterns = append(newPatterns, pattern)
		}
		p.pc++
	}
	if !validForEach {
		return nil, invalidForEachStatement
	}
	// loop for each
	results := make([]interface{}, 0)

	convertedArr, err := convertInterfaceToSlice(val[0])
	if err != nil {
		return nil, err
	}

	for i, _ := range convertedArr {
		val, err := parseBlockPatterns(p, newPatterns, map[string]interface{}{
			loopIndex: i,
		})
		if err != nil {
			return nil, err
		}
		if val != nil && len(val) > 0{
			results = append(results, val...)
		}
	}
	return results, nil
}

// split splits given string(maybe expression) with a separator
func split(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 2 {
		return nil, notEnoughArgsForSplit
	}
	if reflect.TypeOf(extras[0]).Kind() != reflect.String && reflect.TypeOf(extras[1]).Kind() != reflect.String {
		return nil, invalidSplitArgs
	}

	// execute extras[0] in case it contains any built-in or CEL structure
	str, err := p.handleContent(extras[0].(string))
	if err != nil {
		return nil, err
	}

	// execute separator at extras[1]
	val, err := p.handleContent(extras[1].(string))
	if err != nil {
		return nil, err
	}
	if val != nil && len(val) > 0 && reflect.TypeOf(val[0]).Kind() == reflect.String &&
		str != nil && len(str) >0 && reflect.TypeOf(str[0]).Kind() == reflect.String {
		separator := val[0].(string)
		splitStr := strings.Split(str[0].(string), separator)
		return []interface{}{splitStr}, nil
	}
	return nil, invalidSplitArgs
}

// TODO(@kiendn): add function that do specific things such as converting numbers from types to types, etc.

// getDataFromSmc gets data from smc through method and params
func getDataFromSmc(p *Parser, method string, patterns []string) ([]interface{}, error) {

	caller := p.bc.Config().BaseAccount.Address
	contractAddress := p.smartContractAddress
	currentHeader := p.bc.CurrentHeader()
	db := p.bc.DB()

	// get abi from smart contract address, if abi is not found, returns error
	kAbi := db.ReadSmartContractAbi(contractAddress.Hex())
	if kAbi == nil {
		return nil, abiNotFound
	}
	// get packed input from smart contract
	input, err := getPackedInput(p, kAbi, method, patterns)
	if err != nil {
		return nil, err
	}
	// get data from smc using above input
	result, err := callStaticKardiaMasterSmc(caller, *contractAddress, currentHeader, p.bc, input, p.stateDb)
	if err != nil {
		return nil, err
	}
	// base on output convert result
	outputResult, err := GenerateOutputStruct(*kAbi, method)
	if err != nil {
		return nil, err
	}
	// unpack result into output
	if err := kAbi.Unpack(&outputResult, method, result); err != nil {
		return nil, err
	}
	// loop for each field in output. Convert to string and add them into a list
	o := reflect.ValueOf(outputResult)
	return convertOutputToNative(o, kAbi.Methods[method].Outputs)
}

func convertOutputToNative(o reflect.Value, outputs abi.Arguments) ([]interface{}, error) {
	args := make([]interface{}, 0)
	// if o is a primary type, convert it directly
	if o.Kind() != reflect.Interface && o.Kind() != reflect.Ptr {
		v, err := convertToNative(o)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	} else { // otherwise, loop it through outputs and add every field into nestedArgs
		for i, _ := range outputs {
			val := o.Elem().Field(i)
			v, err := convertToNative(val)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
	}
	return args, nil
}

func convertInterfaceToSlice(val interface{}) ([]interface{}, error) {
	if reflect.TypeOf(val).Kind() != reflect.Slice && reflect.TypeOf(val).Kind() != reflect.Array {
		return nil, invalidForEachParam
	}
	results := make([]interface{}, 0)
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
	default:
		return nil, unsupportedType
	}
	return results, nil
}

// convertParams gets data from message based on CEL and then convert returned values based on abi argument types.
func convertParams(p *Parser, arguments abi.Arguments, patterns []string) ([]interface{}, error) {
	if len(arguments) != len(patterns) {
		return nil, paramsArgumentsNotMatch
	}

	abiInputs := make([]interface{}, 0)
	for i, pattern := range patterns {
		vals, err := p.CEL(pattern)
		if err != nil {
			return nil, err
		}

		// vals is an []interface{}. the elements are any types if they are get from params (could be output of calling smc)
		// or string if they are get from message.params (a list of string)
		// if we use argument's types to cast the element. panic might happen and lead to crash.
		// therefore the solution is: if element is string then we check arg's type and cast element to that type based on strconv
		// otherwise add the element to abiInputs without doing anything.

		arg := arguments[i]
		t := arg.Type.Kind.String()
		for _, val := range vals {
			if reflect.TypeOf(val).Kind() != reflect.String {
				abiInputs = append(abiInputs, val)
				continue
			}
			switch t {
			case "string": abiInputs = append(abiInputs, val)
			case "int8":
				// convert val to int based with bitSize = 8
				result, err := strconv.ParseInt(val.(string), 10, 8)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, int8(result))
			case "int16":
				// convert val to int with bitSize = 16
				result, err := strconv.ParseInt(val.(string), 10, 16)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, int16(result))
			case "int32":
				// convert val to int with bitSize = 32
				result, err := strconv.ParseInt(val.(string), 10, 32)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, int32(result))
			case "int64":
				// convert val to int with bitSize = 64
				result, err := strconv.ParseInt(val.(string), 10, 64)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, result)
			case "uint8":
				// convert val to uint based with bitSize = 8
				result, err := strconv.ParseUint(val.(string), 10, 8)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, uint8(result))
			case "uint16":
				// convert val to int with bitSize = 16
				result, err := strconv.ParseUint(val.(string), 10, 16)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, uint16(result))
			case "uint32":
				// convert val to int with bitSize = 32
				result, err := strconv.ParseUint(val.(string), 10, 32)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, uint32(result))
			case "uint64":
				// convert val to int with bitSize = 64
				result, err := strconv.ParseUint(val.(string), 10, 64)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, result)
			case "bool":
				result, err := strconv.ParseBool(val.(string))
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, result)
			case "array", "slice", "ptr":
				typ := arg.Type.Type.String()
				switch {
				case strings.Contains(typ, "uint8") && strings.HasPrefix(typ, "[") && strings.Count(typ, "]") == 1:
					// val is bytes.
					// convert val to bytes.
					bytesValue := []byte(val.(string))
					// get len of bytes by getting the number between "[" and "]"
					lbrace := strings.Index(typ, "[")
					rbrace := strings.Index(typ, "]")
					if typ[lbrace+1:rbrace] != "" { // val can be an array. get the length and validate val.
						lenOfByte, err := strconv.ParseInt(typ[lbrace+1:rbrace], 10, 32)
						if err != nil {
							return nil, err
						}
						// compare the length with bytesValue.
						if int(lenOfByte) != len(bytesValue) {
							return nil, paramValueNotCorrect
						}
					}
					abiInputs = append(abiInputs, bytesValue)
				case typ == "common.Address":
					abiInputs = append(abiInputs, common.HexToAddress(val.(string)))
				case typ == "*big.Int":
					result, _ := big.NewInt(0).SetString(val.(string), 10)
					abiInputs = append(abiInputs, result)
				default:
					return nil, unsupportedType
				}
			default:
				return nil, unsupportedType
			}
		}
	}
	return abiInputs, nil
}

func getPackedInput(p *Parser, kaiAbi *abi.ABI, method string, patterns []string) ([]byte, error) {
	// get method's inputs from kaiAbi
	if _, ok := kaiAbi.Methods[method]; !ok {
		return nil, methodNotFound
	}

	args, err := convertParams(p, kaiAbi.Methods[method].Inputs, patterns)
	if err != nil {
		return nil, err
	}
	input, err := kaiAbi.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	return input, nil
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
	}
	return "", fmt.Errorf("unsupported value type")
}

// callStaticKardiaMasterSmc calls smc and return result in bytes format
func callStaticKardiaMasterSmc(from common.Address, to common.Address, currentHeader *kaiType.Header, chain vm.ChainContext, input []byte, statedb *state.StateDB) (result []byte, err error) {
	context := vm.NewKVMContextFromDualNodeCall(from, currentHeader, chain)
	vmenv := kvm.NewKVM(context, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(MaximumGasToCallStaticFunction))
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}

// GenerateOutputStructs creates structs for all methods from theirs outputs
func GenerateOutputStruct(smcABI abi.ABI, method string) (interface{}, error) {
	for k, v := range smcABI.Methods {
		if k == method {
			return makeStruct(v.Outputs), nil
		}
	}
	return nil, methodNotFound
}

// makeStruct makes a struct from abi arguments
func makeStruct(args abi.Arguments) interface{} {
	var sfs []reflect.StructField
	for i, arg := range args {
		name := arg.Name
		if name == "" {
			name = fmt.Sprintf("name%v", i)
		}
		sf := reflect.StructField{
			Type: arg.Type.Type,
			Name: fmt.Sprintf("%v", strings.Title(name)),
			Tag: reflect.StructTag(fmt.Sprintf(`abi:"%v"`, name)),
		}
		sfs = append(sfs, sf)
	}
	st := reflect.StructOf(sfs)
	so := reflect.New(st)
	return so.Interface()
}
