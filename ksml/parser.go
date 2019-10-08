package ksml

import (
	"fmt"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/state"
	message "github.com/kardiachain/go-kardia/ksml/proto"
	"github.com/kardiachain/go-kardia/lib/common"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"reflect"
	"strings"
)

type Parser struct {
	bc base.BaseBlockChain
	stateDb *state.StateDB
	smartContractAddress *common.Address
	globalPatterns []string
	globalMessage *message.EventMessage
	globalParams interface{}
	userDefinedFunction map[string][]string
	userDefinedVariables map[string]interface{}
	pc int // program counter
}

func NewParser(bc base.BaseBlockChain, stateDb *state.StateDB, smartContractAddress *common.Address, globalPatterns []string, globalMessage *message.EventMessage) *Parser {
	return &Parser{
		bc:                  bc,
		stateDb:             stateDb,
		smartContractAddress: smartContractAddress,
		globalPatterns:      globalPatterns,
		globalMessage:       globalMessage,
		globalParams:        make([]interface{}, 0),
		userDefinedFunction: make(map[string][]string),
		userDefinedVariables: make(map[string]interface{}),
		pc: 0,
	}
}

func addPrimitiveIdent(name string, kind reflect.Kind) *expr.Decl {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return decls.NewIdent(name, decls.Int, nil)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return decls.NewIdent(name, decls.Uint, nil)
	case reflect.String:
		return decls.NewIdent(name, decls.String, nil)
	case reflect.Bool:
		return decls.NewIdent(name, decls.Bool, nil)
	case reflect.Array, reflect.Slice:
		return decls.NewIdent(name, decls.Dyn, nil)
	default:
		return nil
	}
}


// CEL reads source and get value based Common Expression Language
func (p *Parser)CEL(src string) ([]interface{}, error) {
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
				evalArg[globalMessage] = p.globalMessage
			case globalParams:
				evalArg[globalParams] = p.globalParams
			}
		}
	}

	// add user defined variable
	if len(p.userDefinedVariables) > 0 {
		for k, v := range p.userDefinedVariables {
			if strings.Contains(src, k) {
				ident := addPrimitiveIdent(k, reflect.TypeOf(v).Kind())
				if ident != nil {
					declarations = append(declarations, ident)
					evalArg[k] = v
				}
			}
		}
	}

	// init new env
	env, err := cel.NewEnv(
		cel.Types(p.globalMessage),
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

// getPrefix reads content to get prefix if any, if prefix exists, then it returns method and a list of params
func (p *Parser)getPrefix(content string) (string, string, []string, error) {
	if strings.Contains(content, prefixSeparator) {
		splitContent := strings.Split(content, prefixSeparator)
		if len(splitContent) > 2 {
			return "", "", nil, invalidExpression
		}
		prefix := splitContent[0]
		method := splitContent[1]

		// check if method contains methodName(...) or not. if not return invalid method format
		if strings.Count(method, "(") != strings.Count(method, ")") || !strings.HasSuffix(method, ")") {
			return "", "", nil, invalidMethodFormat
		}

		// check if prefix is in predefinedPrefix
		for _, pre := range predefinedPrefix {
			if prefix != pre {
				continue
			}
			// otherwise start getting method and params then returns those values
			firstIndex := strings.Index(method, "(")
			methodParams := method[firstIndex+1:len(method)-1]
			method = method[0:firstIndex]
			if methodParams == "" { // method has not any param
				return prefix, method, nil, nil
			}
			methodParams = strings.ReplaceAll(methodParams, " ", "")

			// Note: nested function is not allowed
			// check if string has special paramsSeparator or not. replace with temp
			temp := "{temporary}"
			specialParamsSeparator := fmt.Sprintf("\"%v\"", paramsSeparator)
			if strings.Contains(methodParams, specialParamsSeparator) {
				methodParams = strings.ReplaceAll(methodParams, specialParamsSeparator, temp)
			}
			params := strings.Split(methodParams, paramsSeparator)
			for i, _ := range params {
				if params[i] == temp {
					params[i] = specialParamsSeparator
				}
			}
			return prefix, method, params, nil
		}
	}
	return "", content, nil, nil
}

// applyPredefinedFunction applies predefined function, including: fn (built-in function) and smc (trigger smc function)
func (p *Parser)applyPredefinedFunction(prefix, method string, patterns []string) ([]interface{}, error) {
	switch prefix {
	case builtInFn: // execute predefined function
		// add patterns (as method params), message and params (global params) to extras and pass to built-in function
		extras := make([]interface{}, 0)
		for _, p := range patterns {
			extras = append(extras, p)
		}
		return BuiltInFuncMap[method](p, extras...)
	case builtInSmc: // get data by getting value from smc
		return getDataFromSmc(p, method, patterns)
	}
	return nil, nil
}

// ParseParam parses param as a string using CEL if it has ${exp} format, otherwise returns it as a string value
// obj must be a protobuf object
// pkg is obj's name which is defined in protobuf
func (p *Parser)ParseParams() error {
	if len(p.globalPatterns) == 0 {
		return sourceIsEmpty
	}
	for p.pc < len(p.globalPatterns) {
		pattern := p.globalPatterns[p.pc]
		var val []interface{}
		var err error
		// if src is greater or equals minLength and has structure ${...} then CEL is applied
		if len(pattern) >= elMinLength && strings.HasPrefix(pattern, "${") && strings.HasSuffix(pattern, "}") {
			content := pattern[2:len(pattern)-1]
			val, err = p.handleContent(content)
			if err != nil {
				return err
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
						p.globalParams = append(p.globalParams.([]interface{}), val[0:len(val)-1]...)
						p.pc++
						continue
					case signalReturn:
						p.globalParams = append(p.globalParams.([]interface{}), val[0:len(val)-1]...)
						return nil
					case signalStop:
						return stopSignal
					}
				}
			}
			p.globalParams = append(p.globalParams.([]interface{}), val...)
		}
		p.pc++
	}
	return nil
}

func (p *Parser)handleContent(content string) ([]interface{}, error) {
	// check if content contains any predefined prefix
	prefix, method, patterns, err := p.getPrefix(content)
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
