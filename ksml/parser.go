/*
 *  Copyright 2019 KardiaChain
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

package ksml

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"

	dualMsg "github.com/kardiachain/go-kardiamain/dualnode/message"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/state"
	message "github.com/kardiachain/go-kardiamain/ksml/proto"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
)

type Parser struct {
	ProxyName            string                                                                // name of proxy that is using parser (NEO, ETH, TRX)
	PublishEndpoint      string                                                                // endpoint that message will be published to, in case publish action is used
	PublishFunction      func(endpoint string, topic string, msg dualMsg.TriggerMessage) error // function is used for publish message to client chain
	Bc                   base.BaseBlockChain                                                   // kardia blockchain
	TxPool               *tx_pool.TxPool                                                       // kardia tx pool is used when smc:trigger is called.
	StateDb              *state.StateDB
	SmartContractAddress *common.Address        // master smart contract
	GlobalPatterns       []string               // globalPatterns is a list of actions that parser will read through
	GlobalMessage        *message.EventMessage  // globalMessage is a global variables passed as type proto.Message
	GlobalParams         []interface{}          // all returned value while executing globalPatterns are stored here
	UserDefinedFunction  map[string]*function   // before parse globalPatterns, parser will read through it once to get all defined functions
	UserDefinedVariables map[string]interface{} // all variables defined in globalPatterns will be added here while parser reads through it
	Pc                   int                    // program counter is used to count and get current read position in globalPatterns
	Nonce                uint64
	CanTrigger           bool
	mtx                  sync.Mutex
}

func NewParser(proxyName, publishedEndpoint string, publishFunction func(endpoint string, topic string, msg dualMsg.TriggerMessage) error,
	bc base.BaseBlockChain, txPool *tx_pool.TxPool,
	smartContractAddress *common.Address, globalPatterns []string, globalMessage *message.EventMessage, canTrigger bool) *Parser {
	stateDb := txPool.State()
	return &Parser{
		ProxyName:            proxyName,
		PublishEndpoint:      publishedEndpoint,
		PublishFunction:      publishFunction,
		Bc:                   bc,
		TxPool:               txPool,
		StateDb:              stateDb,
		SmartContractAddress: smartContractAddress,
		GlobalPatterns:       globalPatterns,
		GlobalMessage:        globalMessage,
		GlobalParams:         make([]interface{}, 0),
		UserDefinedFunction:  make(map[string]*function),
		UserDefinedVariables: make(map[string]interface{}),
		Nonce:                0,
		Pc:                   0,
		CanTrigger:           canTrigger,
	}
}

func addPrimitiveIdent(name string, v interface{}) (interface{}, *expr.Decl) {

	if strings.Contains(reflect.ValueOf(v).Type().String(), "big.Int") {
		v = v.(*big.Int).String()
	} else if strings.Contains(reflect.ValueOf(v).Type().String(), "big.Float") {
		r, _ := v.(*big.Float)
		v = r.String()
	}

	kind := reflect.TypeOf(v).Kind()
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v, decls.NewIdent(name, decls.Int, nil)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v, decls.NewIdent(name, decls.Uint, nil)
	case reflect.Float32, reflect.Float64:
		return v, decls.NewIdent(name, decls.Double, nil)
	case reflect.String:
		return v, decls.NewIdent(name, decls.String, nil)
	case reflect.Bool:
		return v, decls.NewIdent(name, decls.Bool, nil)
	case reflect.Array, reflect.Slice, reflect.Ptr:
		return v, decls.NewIdent(name, decls.Dyn, nil)
	default:
		return v, nil
	}
}

// CEL reads source and get value based Common Expression Language
func (p *Parser) CEL(src string) ([]interface{}, error) {
	if src == "" {
		return nil, sourceIsEmpty
	}
	evalArg := make(map[string]interface{})
	// get globalVars if they are found in src
	declarations := make([]*expr.Decl, 0)
	for k, v := range globalVars {
		if strings.Contains(src, k) {
			declarations = append(declarations, v)
			switch k {
			case globalMessage:
				evalArg[globalMessage] = p.GlobalMessage
			case globalParams:
				params := make([]interface{}, 0)
				for _, p := range p.GlobalParams {
					if reflect.ValueOf(p).Type().String() == "*big.Int" {
						params = append(params, p.(*big.Int).String())
					} else if reflect.ValueOf(p).Type().String() == "*big.Float" {
						floatValue, _ := p.(*big.Float)
						params = append(params, floatValue.String())
					} else {
						params = append(params, p)
					}
				}
				evalArg[globalParams] = params
			case globalContractAddress:
				evalArg[globalContractAddress] = p.SmartContractAddress.Hex()
			case globalProxyName:
				evalArg[globalProxyName] = p.ProxyName
			}
		}
	}

	// add user defined variable
	if len(p.UserDefinedVariables) > 0 {
		for k, v := range p.UserDefinedVariables {
			if strings.Contains(src, k) {
				val, ident := addPrimitiveIdent(k, v)
				if ident != nil {
					declarations = append(declarations, ident)
					evalArg[k] = val
				}
			}
		}
	}

	// init new env
	env, err := cel.NewEnv(
		cel.Types(p.GlobalMessage),
		cel.Declarations(declarations...),
	)
	if err != nil {
		return nil, err
	}

	// parse src
	ast, iss := env.Parse(src)
	if iss != nil && iss.Err() != nil {
		return nil, iss.Err()
	}

	// check parse
	c, iss := env.Check(ast)
	if iss != nil && iss.Err() != nil {
		if strings.Contains(iss.Err().Error(), "undeclared reference to") {
			// sometime there is bare string is passed into this function
			// while CEL does not accept this therefore this error is returned
			// check if it is occurred, return that string without doing CEL
			return []interface{}{types.String(src)}, nil
		}
		return nil, iss.Err()
	}

	prg, err := env.Program(c)
	if err != nil {
		return nil, err
	}

	out, _, err := prg.Eval(evalArg)
	if err != nil {
		return nil, err
	}
	return []interface{}{out.Value()}, nil
}

func (p *Parser) GetNonce() uint64 {
	nonce := p.TxPool.Nonce(p.Bc.Config().BaseAccount.Address)

	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.Nonce < nonce {
		p.Nonce = nonce
	}
	return p.Nonce
}

func hasBuiltIn(content string) bool {
	fnPrefix := fmt.Sprintf("%v%v", builtInFn, prefixSeparator)
	smcPrefix := fmt.Sprintf("%v%v", builtInSmc, prefixSeparator)
	return len(content) > 6 && (strings.HasPrefix(content, fnPrefix) || strings.HasPrefix(content, smcPrefix))
}

// GetPrefix reads content to get prefix if any, if prefix exists, then it returns method and a list of params
func (p *Parser) GetPrefix(content string) (string, string, []string, error) {
	invalidBuiltInFuncSync := fmt.Errorf("invalid built-in function syntax")
	if hasBuiltIn(content) {
		// content has built-in function.
		// get method from at position 3 to the first position of "("
		firstParen := strings.Index(content, "(")
		if firstParen < 0 || !strings.HasSuffix(content, ")") {
			return "", "", nil, invalidBuiltInFuncSync
		}
		prefix := content[:strings.Index(content, prefixSeparator)]
		method := content[strings.Index(content, prefixSeparator)+1 : firstParen]
		params := make([]string, 0)

		// jump content to firstParen+1 to len(content)-1
		content := content[firstParen+1 : len(content)-1]
		if content == "" {
			return prefix, method, params, nil
		}

		// loop until idx < 0
		for true {
			// remove white space
			content = strings.ReplaceAll(content, " ", "")
			idx := strings.Index(content, ",")
			if idx < 0 {
				params = append(params, content)
				break
			}
			nested := content[0:idx]
			// if nested contains prefix then count number of paren and thesis in content until they are balanced to get whole built-in function definition
			if hasBuiltIn(nested) {
				i := strings.Index(content, "(")
				if i < 0 {
					return "", "", nil, invalidBuiltInFuncSync
				}
				count := 1
				// loop from the next index
				for i < len(content)-1 {
					i++
					if content[i] == ')' {
						count--
					} else if content[i] == '(' {
						count++
					}
					if count == 0 {
						break
					}
				}
				if count != 0 {
					return "", "", nil, invalidBuiltInFuncSync
				}
				if i+1 == len(content) {
					// the whole new content is a built-in function
					// do nothing, return content as method's params
					params = append(params, content)
					break
				}
				if content[i+1] != ',' || i+2 > len(content) {
					// next index is not comma, means that it is wrong syntax
					// after thesis(')') and comma(',') there must be at least another param.
					// therefore check if len(content) > i + 2 or not.
					return "", "", nil, invalidBuiltInFuncSync
				}
				params = append(params, content[:i+1])

				// update content to i+2
				content = content[i+2:]
				continue
			}
			params = append(params, nested)

			if len(content) < idx+1 {
				break
			}
			content = content[idx+1:]
		}
		return prefix, method, params, nil
	}
	return "", content, nil, nil
}

// applyPredefinedFunction applies predefined function, including: fn (built-in function) and smc (trigger smc function)
func (p *Parser) applyPredefinedFunction(prefix, method string, patterns []string) ([]interface{}, error) {
	switch prefix {
	case builtInFn, builtInSmc: // execute predefined function
		// add patterns (as method params), message and params (global params) to extras and pass to built-in function
		extras := make([]interface{}, 0)
		for _, p := range patterns {
			extras = append(extras, p)
		}
		return BuiltInFuncMap[method](p, extras...)
	}
	return nil, nil
}

func (p *Parser) addFunction() error {
	if len(p.GlobalPatterns) == 0 {
		return sourceIsEmpty
	}
	for p.Pc < len(p.GlobalPatterns) {
		pattern := p.GlobalPatterns[p.Pc]
		if strings.Contains(pattern, defineFunc) {
			_, err := p.handleContent(pattern[2 : len(pattern)-1])
			if err != nil {
				return err
			}
			// reset pc and start recursively again
			p.Pc = 0
			if err := p.addFunction(); err != nil {
				return err
			}
		}
		p.Pc += 1
	}
	// reset pc
	p.Pc = 0
	return nil
}

// ParseParam parses param as a string using CEL if it has ${exp} format, otherwise returns it as a string value
// obj must be a protobuf object
// pkg is obj's name which is defined in protobuf
func (p *Parser) ParseParams() error {

	// defer panic
	defer func() {
		if err := recover(); err != nil {
			log.Error("panic", "err", err)
		}
	}()

	if len(p.GlobalPatterns) == 0 {
		return sourceIsEmpty
	}

	// check and add userDefinedFunction
	if err := p.addFunction(); err != nil {
		return err
	}

	for p.Pc < len(p.GlobalPatterns) {
		pattern := p.GlobalPatterns[p.Pc]
		var val []interface{}
		var err error
		// if src is greater or equals minLength and has structure ${...} then CEL is applied
		if len(pattern) >= elMinLength && strings.HasPrefix(pattern, "${") && strings.HasSuffix(pattern, "}") {
			content := pattern[2 : len(pattern)-1]
			val, err = p.handleContent(content)
			if err != nil {
				return fmt.Errorf("error while handling content at line %v - %v", p.Pc, err)
			}
		} else {
			val = []interface{}{pattern}
		}
		if val != nil && len(val) > 0 {
			// evaluate signals
			lastEl := val[len(val)-1]
			if reflect.TypeOf(lastEl).Kind() == reflect.String {
				if _, ok := signals[lastEl.(string)]; ok {
					switch lastEl.(string) {
					case signalContinue:
						p.GlobalParams = append(p.GlobalParams, val[0:len(val)-1]...)
						p.Pc++
						continue
					case signalReturn:
						p.GlobalParams = append(p.GlobalParams, val[0:len(val)-1]...)
						return nil
					case signalStop:
						return stopSignal
					}
				}
			}
			p.GlobalParams = append(p.GlobalParams, val...)
		}
		p.Pc++
	}
	return nil
}

func (p *Parser) handleContents(contents []interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0)
	for _, content := range contents {
		data, err := p.handleContent(content.(string))
		if err != nil {
			return nil, err
		}
		results = append(results, data[0])
	}
	if len(results) != len(contents) {
		return nil, fmt.Errorf("problem occurred while handling content, len output does not match with len input")
	}
	return results, nil
}

func (p *Parser) handleContent(content string) ([]interface{}, error) {
	// check if content contains any predefined prefix
	prefix, method, patterns, err := p.GetPrefix(content)
	var val []interface{}
	if err != nil {
		return nil, err
	}
	if prefix == "" {
		// apply CEL to content
		val, err = p.CEL(content)
	} else {
		// apply predefined function
		val, err = p.applyPredefinedFunction(prefix, method, patterns)
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (p *Parser) GetGlobalParams() []interface{} {
	return p.GlobalParams
}

func (p *Parser) GetParams() ([]string, error) {
	results := make([]string, 0)
	for _, param := range p.GlobalParams {
		v, err := InterfaceToString(param)
		if err != nil {
			return nil, err
		}
		results = append(results, v)
	}
	return results, nil
}
