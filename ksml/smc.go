package ksml

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

func generateInput(p *Parser, extras ...interface{}) (string, *abi.ABI, *common.Address, *types.Header, []byte, error) {
	if len(extras) == 0 {
		return "", nil, nil, nil, nil, sourceIsEmpty
	}
	method := extras[0].(string)
	patterns := make([]string, 0)
	if len(extras) > 1 {
		for _, pattern := range extras[1:] {
			// handle content of arg
			patterns = append(patterns, pattern.(string))
		}
	}
	caller := p.Bc.Config().BaseAccount.Address
	contractAddress := p.SmartContractAddress
	currentHeader := p.Bc.CurrentHeader()
	db := p.Bc.DB()

	// get abi from smart contract address, if abi is not found, returns error
	kAbi := db.ReadSmartContractAbi(contractAddress.Hex())
	if kAbi == nil {
		return "", nil, nil, nil, nil, abiNotFound
	}
	// get packed input from smart contract
	input, err := getPackedInput(p, kAbi, method, patterns)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	return method, kAbi, &caller, currentHeader, input, nil
}

// getDataFromSmc gets data from smc through method and params
func GetDataFromSmc(p *Parser, extras ...interface{}) ([]interface{}, error) {
	method, kAbi, caller, currentHeader, input, err := generateInput(p, extras...)
	if err != nil {
		return nil, err
	}
	// get data from smc using above input
	result, err := callStaticKardiaMasterSmc(*caller, *p.SmartContractAddress, currentHeader, p.Bc, input, p.StateDb)
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

// triggerSmc triggers an smc call by creating tx and send to txPool.
func triggerSmc(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if !p.CanTrigger {
		return nil, fmt.Errorf("trigger smc is not allowed")
	}
	_, _, caller, currentHeader, input, err := generateInput(p, extras...)
	if err != nil {
		return nil, err
	}
	gas, err := EstimateGas(*caller, *p.SmartContractAddress, currentHeader, p.Bc, p.StateDb, input)
	if err != nil {
		return nil, err
	}
	// otherwise use gas to create new transaction and add to txPool
	tx, err := GenerateSmcCall(p.GetNonce(), &p.Bc.Config().BaseAccount.PrivateKey, *p.SmartContractAddress, input, gas)
	if err != nil {
		return nil, err
	}

	// add tx to txPool
	if err := p.TxPool.AddTx(tx); err != nil {
		return nil, err
	}

	// update nonce
	p.Nonce += 1

	// return tx
	return []interface{}{tx.Hash().Hex()}, nil
}

// GenerateSmcCall generates tx which call a smart contract's method
// if isIncrement is true, nonce + 1 to prevent duplicate nonce if generateSmcCall is called twice.
func GenerateSmcCall(nonce uint64, senderKey *ecdsa.PrivateKey, address common.Address, input []byte, gasLimit uint64) (*types.Transaction, error) {
	return types.SignTx(types.NewTransaction(
		nonce,
		address,
		big.NewInt(0),
		gasLimit,
		big.NewInt(1),
		input,
	), senderKey)
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

// ConvertParams gets data from message based on CEL and then convert returned values based on abi argument types.
func ConvertParams(p *Parser, arguments abi.Arguments, patterns []string) ([]interface{}, error) {
	if len(arguments) != len(patterns) {
		return nil, paramsArgumentsNotMatch
	}

	abiInputs := make([]interface{}, 0)
	for i, pattern := range patterns {
		vals, err := p.handleContent(pattern)
		if err != nil {
			return nil, err
		}

		// vals is an []interface{}. the elements are any types if they are get from params (could be output of calling smc)
		// or string if they are get from message.params (a list of string)
		// if we use argument's types to cast the element. panic might happen and lead to crash.
		// therefore the solution is: if element is string then we check arg's type and cast element to that type based on strconv
		// otherwise add the element to abiInputs without doing anything.

		arg := arguments[i]
		t := arg.Type.Kind
		for _, val := range vals {
			if reflect.TypeOf(val).Kind() != reflect.String {
				abiInputs = append(abiInputs, val)
				continue
			}
			switch t {
			case reflect.String: abiInputs = append(abiInputs, val)
			case reflect.Int8:
				// convert val to int based with bitSize = 8
				result, err := strconv.ParseInt(val.(string), 10, 8)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, int8(result))
			case reflect.Int16:
				// convert val to int with bitSize = 16
				result, err := strconv.ParseInt(val.(string), 10, 16)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, int16(result))
			case reflect.Int32:
				// convert val to int with bitSize = 32
				result, err := strconv.ParseInt(val.(string), 10, 32)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, int32(result))
			case reflect.Int64:
				// convert val to int with bitSize = 64
				result, err := strconv.ParseInt(val.(string), 10, 64)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, result)
			case reflect.Uint8:
				// convert val to uint based with bitSize = 8
				result, err := strconv.ParseUint(val.(string), 10, 8)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, uint8(result))
			case reflect.Uint16:
				// convert val to int with bitSize = 16
				result, err := strconv.ParseUint(val.(string), 10, 16)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, uint16(result))
			case reflect.Uint32:
				// convert val to int with bitSize = 32
				result, err := strconv.ParseUint(val.(string), 10, 32)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, uint32(result))
			case reflect.Uint64:
				// convert val to int with bitSize = 64
				result, err := strconv.ParseUint(val.(string), 10, 64)
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, result)
			case reflect.Bool:
				result, err := strconv.ParseBool(val.(string))
				if err != nil {
					return nil, err
				}
				abiInputs = append(abiInputs, result)
			case reflect.Array, reflect.Slice, reflect.Ptr:
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

	args, err := ConvertParams(p, kaiAbi.Methods[method].Inputs, patterns)
	if err != nil {
		return nil, err
	}
	input, err := kaiAbi.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	return input, nil
}

// callStaticKardiaMasterSmc calls smc and return result in bytes format
func callStaticKardiaMasterSmc(from common.Address, to common.Address, currentHeader *types.Header, chain vm.ChainContext, input []byte, statedb *state.StateDB) (result []byte, err error) {
	ctx := vm.NewKVMContextFromDualNodeCall(from, currentHeader, chain)
	vmenv := kvm.NewKVM(ctx, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(MaximumGasToCallFunction))
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}

// EstimateGas estimates spent in order to
func EstimateGas(from common.Address, to common.Address, currentHeader *types.Header, bc base.BaseBlockChain, stateDb *state.StateDB, input []byte) (uint64, error){
	// Create new call message
	msg := types.NewMessage(from, &to, 0, big.NewInt(0), uint64(MaximumGasToCallFunction), big.NewInt(1), input, false)
	// Create a new context to be used in the KVM environment
	vmContext := vm.NewKVMContext(msg, currentHeader, bc)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	kaiVm := kvm.NewKVM(vmContext, stateDb, kvm.Config{
		IsZeroFee: bc.ZeroFee(),
	})
	defer kaiVm.Cancel()
	// Apply the transaction to the current state (included in the env)
	gp := new(types.GasPool).AddGas(common.MaxUint64)
	_, gas, _, err := bc.ApplyMessage(kaiVm, msg, gp)
	if err != nil {
		return 0, err
	}
	// If the timer caused an abort, return an appropriate error message
	if kaiVm.Cancelled() {
		return 0, fmt.Errorf("execution aborted")
	}
	return gas, nil
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

func getInputs(smcABI abi.ABI, method string) *abi.Arguments {
	for k, v := range smcABI.Methods {
		if k == method {
			return &v.Inputs
		}
	}
	return nil
}

// GenerateInputStructs creates structs for all methods from theirs inputs
func GenerateInputStruct(smcABI abi.ABI, input []byte) (*abi.Method, interface{}, error) {
	method, err := smcABI.MethodById(input)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range smcABI.Methods {
		if k == method.Name {
			return method, makeStruct(v.Inputs), nil
		}
	}
	return nil, nil, fmt.Errorf("method not found")
}

func GetMethodAndParams(smcABI abi.ABI, input []byte) (string, []string, error) {
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
		v, err := InterfaceToString(obj.Elem().Field(i).Interface())
		if err != nil {
			return "", nil, err
		}
		args = append(args, v)
	}
	return method.Name, args, nil
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
