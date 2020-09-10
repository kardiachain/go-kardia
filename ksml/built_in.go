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
	"github.com/kardiachain/go-kardiamain/dualnode/message"
	"reflect"
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
		defineFunc: defineFunction,
		endDefineFunc: emptyFunc,
		callFunc: callFunction,
		getData: GetDataFromSmc,
		trigger: triggerSmc,
		publish: publishFunc,
		compare: cmpFunc,
		mul: Mul,
		div: Div,
		toInt: SetInt,
		toFloat: SetFloat,
		exp: Exp,
		format: FormatFloat,
		round: Round,
		replaceFunc: Replace,
	}
}

func emptyFunc(p *Parser, extras ...interface{}) ([]interface{}, error) {
	return nil, fmt.Errorf("this function cannot be called")
}

func getCurrentTimeStamp(p *Parser, extras ...interface{}) ([]interface{}, error) {
	now := time.Now().Unix()
	return []interface{}{now}, nil
}

func getCurrentBlockHeight(p *Parser, extras ...interface{}) ([]interface{}, error) {
	height := p.Bc.CurrentBlock().Height()
	return []interface{}{int64(height)}, nil
}

func pong(p *Parser, extras ...interface{}) ([]interface{}, error) {
	return []interface{}{"pong"}, nil
}

// addVar adds a variable into parser.UserDefinedVariables. extras must has len=3 which [0] is varName, [1] is varType, [2] is value
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
	p.UserDefinedVariables[varName] = v
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
	currentPos := p.Pc
	patternBlocks := make(map[string][]string)
	newPatterns := make([]string, 0)
	key := condition
	listCond := make([]string, 0)
	validIfStatement := false

	for _, pattern := range p.GlobalPatterns[currentPos+1:] {
		if strings.Contains(pattern, name) && (strings.Contains(pattern, endIf) ||
			strings.Contains(pattern, elif) || strings.Contains(pattern, el)) {
			patternBlocks[key] = newPatterns
			listCond = append(listCond, key)
			_, method, results, err := p.GetPrefix(strings.ReplaceAll(strings.ReplaceAll(pattern, "}", ""), "${", ""))
			if err != nil {
				return nil, err
			}
			if method == el {
				key = fmt.Sprintf("%v(%v)", el, name)
			} else if method == endIf {
				// move program counter to the next position then break
				p.Pc++
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
		p.Pc++
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
	newParser := NewParser(p.ProxyName, p.PublishEndpoint, p.PublishFunction, p.Bc, p.TxPool, p.SmartContractAddress, patterns, p.GlobalMessage, p.CanTrigger)
	// add all definedVariables in p in overwrite cases.
	for k, v := range p.UserDefinedVariables {
		newParser.UserDefinedVariables[k] = v
	}

	if extrasVar != nil {
		for k, v := range extrasVar {
			newParser.UserDefinedVariables[k] = v
		}
	}

	// add all UserDefinedFunction in p
	for k, v := range p.UserDefinedFunction {
		newParser.UserDefinedFunction[k] = v
	}

	err := newParser.ParseParams()
	if err != nil {
		return nil, err
	}
	// update updated variables in newParser
	for k, v := range newParser.UserDefinedVariables {
		if _, ok := p.UserDefinedVariables[k]; ok {
			p.UserDefinedVariables[k] = v
		}
	}
	return newParser.GlobalParams, nil
}

// forEach loops through a given list variables and execute all logics inside forEach(name, var, indexVar)...endForEach(name) pair.
func forEach(p *Parser, extras ...interface{}) ([]interface{}, error) {
	// extras must have 2 elements: first element is the name of for loop which is used to find forEachEnd.
	// second element must be an array or a slice.
	if len(extras) != 3 {
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
	index := extras[2].(string)
	newPatterns := make([]string, 0)
	validForEach := false
	// loop GlobalPatterns from current position until we find
	for _, pattern := range p.GlobalPatterns[p.Pc+1:] {
		if strings.Contains(pattern, name) && strings.Contains(pattern, endForEach) {
			validForEach = true
		} else {
			newPatterns = append(newPatterns, pattern)
		}
		p.Pc++
	}
	if !validForEach {
		return nil, invalidForEachStatement
	}
	// loop for each
	results := make([]interface{}, 0)

	convertedArr, err := interfaceToSlice(val[0])
	if err != nil {
		return nil, err
	}

	for i, _ := range convertedArr {
		val, err := parseBlockPatterns(p, newPatterns, map[string]interface{}{
			index: i,
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

// Replace finds and replaces string that match given pattern with new pattern
func Replace(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 3 {
		return nil, fmt.Errorf("not enough arguments for replace function, expect 3 got %v", len(extras))
	}
	// execute extras[0] in case it contains any built-in or CEL structure
	vals, err := p.handleContents(extras)
	if err != nil {
		return nil, err
	}
	str, err := InterfaceToString(vals[0])
	if err != nil {
		return nil, err
	}
	old, err := InterfaceToString(vals[1])
	if err != nil {
		return nil, err
	}
	newStr, err := InterfaceToString(vals[2])
	if err != nil {
		return nil, err
	}

	str = strings.Replace(str, old, newStr, -1)
	return []interface{}{str}, nil
}

// defineFunction defines function and add to UserDefinedFunction
func defineFunction(p *Parser, extras ...interface{}) ([]interface{}, error) {
	method := extras[0].(string)
	args := make([]string, 0)
	if len(extras) > 1 {
		for _, arg := range extras[1:] {
			args = append(args, arg.(string))
		}
	}
	f := &function{
		name: method,
		args: args,
		patterns: make([]string, 0),
	}
	startPos := p.Pc
	endPos := 0

	for _, pattern := range p.GlobalPatterns[startPos+1:] {
		p.Pc += 1
		if strings.Contains(pattern, fmt.Sprintf("%v(%v)", endDefineFunc, method)) {
			endPos = p.Pc
			break
		}
		f.patterns = append(f.patterns, pattern)
	}
	if endPos == 0 {
		// endDefineFunc is not found
		return nil, invalidDefineFunc
	}

	// add function to UserDefinedFunc if method name does not exist
	if _, ok := p.UserDefinedFunction[method]; !ok {
		p.UserDefinedFunction[method] = f
	}

	// remove patterns from startPos to endPos
	newPatterns := p.GlobalPatterns[0:startPos]
	newPatterns = append(newPatterns, p.GlobalPatterns[endPos+1:]...)
	p.GlobalPatterns = newPatterns

	return nil, nil
}

// callFunction calls function while function's name must exist in UserDefinedFunction.
func callFunction(p *Parser, extras ...interface{}) ([]interface{}, error) {
	method := extras[0].(string)
	args := make([]interface{}, 0)
	if len(extras) > 1 {
		args = append(args, extras[1:]...)
	}
	if _, ok := p.UserDefinedFunction[method]; !ok {
		return nil, methodNotFound
	}
	f := p.UserDefinedFunction[method]
	// validate length of args
	if len(args) != len(f.args) {
		return nil, invalidVariables
	}
	vars := make(map[string]interface{})
	for i, arg := range f.args {
		// handle content of arg before adding to vars
		val, err := p.handleContent(args[i].(string))
		if err != nil {
			return nil, err
		}
		if len(val) > 0 {
			vars[arg] = val[0]
		}
	}
	results, err := parseBlockPatterns(p, f.patterns, vars)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func getTriggerMessage(p *Parser, input []interface{}) (*message.TriggerMessage, error){
	if len(input) != 3 {
		return nil, fmt.Errorf("invalid input in getTriggerMessage")
	}
	// handleContent for contractAddress
	contractStr, err := InterfaceToString(input[0])
	if err != nil {
		return nil, err
	}
	val, err := p.handleContent(contractStr)
	if err != nil {
		return nil, err
	}
	if reflect.TypeOf(val[0]).Kind() != reflect.String {
		return nil, fmt.Errorf("contractAddress must be string")
	}
	contractAddress, err := InterfaceToString(val[0])
	if err != nil {
		return nil, err
	}

	// handleContent for method
	methodStr, err := InterfaceToString(input[1])
	if err != nil {
		return nil, err
	}
	val, err = p.handleContent(methodStr)
	if err != nil {
		return nil, err
	}
	if reflect.TypeOf(val[0]).Kind() != reflect.String {
		return nil, fmt.Errorf("method must be string")
	}
	method, err := InterfaceToString(val[0])
	if err != nil {
		return nil, err
	}

	// handleContent for callbacks
	// callbacks may be a slice/array if they are returned from function called or string
	val = make([]interface{}, 0)
	if reflect.ValueOf(input[2]).Kind() == reflect.Slice || reflect.ValueOf(input[2]).Kind() == reflect.Array {
		val, err = interfaceToSlice(input[2])
		if err != nil {
			return nil, err
		}
	} else if reflect.ValueOf(input[2]).Kind() == reflect.String {
		v, err := p.handleContent(input[2].(string))
		if err != nil {
			return nil, err
		}
		if reflect.TypeOf(v[0]).Kind() != reflect.Slice && reflect.TypeOf(v[0]).Kind() != reflect.Array {
			return nil, fmt.Errorf("params must be a list")
		}
		// otherwise val = v[0]
		val, err = interfaceToSlice(v[0])
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid callback format")
	}
	params := make([]string, 0)
	for _, v := range val {
		str, err := InterfaceToString(v)
		if err != nil {
			return nil, err
		}
		params = append(params, str)
	}
	return &message.TriggerMessage{
		ContractAddress:      contractAddress,
		MethodName:           method,
		Params:               params,
		CallBacks:            nil,
	}, nil
}

// publishFunc publish triggerMessage to target client.
// extras must include targetContractAddress, method, params (list), []callBack (callback is a list of triggerMessage)
func publishFunc(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) < 3 {
		return nil, fmt.Errorf("not enough arguments in publish function")
	}
	msg, err := getTriggerMessage(p, extras[0:3])
	if err != nil {
		return nil, err
	}
	callBacks := make([]*message.TriggerMessage, 0)
	if len(extras) == 4 {
		// extras[3] contains a list of callback.
		// use CEL to convert into a list of expression
		val, err := p.handleContent(extras[3].(string))
		if err != nil {
			return nil, err
		}
		vals, err := interfaceToSlice(val[0])
		if err != nil {
			return nil, err
		}
		// loop through a list of expression and get callBack by calling getTriggerMessage
		for _, v := range vals {
			element, err := interfaceToSlice(v)
			if err != nil {
				return nil, err
			}
			cb, err := getTriggerMessage(p, element)
			if err != nil {
				return nil, err
			}
			callBacks = append(callBacks, cb)
		}
	}
	msg.CallBacks = callBacks
	if err := p.PublishFunction(p.PublishEndpoint, KARDIA_CALL, *msg); err != nil {
		return nil, err
	}
	return nil, nil
}

// cmpFunc compare 2 variables, if equal then return extras[2] otherwise return extras[3]
func cmpFunc(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 4 {
		return nil, fmt.Errorf("not enough arguments for cmp function, expect 4 got %v", len(extras))
	}
	str1, err := InterfaceToString(extras[0])
	if err != nil {
		return nil, err
	}
	v1, err := p.handleContent(str1)
	if err != nil {
		return nil, err
	}
	str2, err := InterfaceToString(extras[1])
	if err != nil {
		return nil, err
	}
	v2, err := p.handleContent(str2)
	if err != nil {
		return nil, err
	}
	value1, value2 := reflect.ValueOf(v1[0]), reflect.ValueOf(v2[0])
	if value1.Type() != value2.Type() {
		return nil, fmt.Errorf("cannot compare type %v with %v", value1.Type().String(), value1.Type().String())
	}
	if value1 == value2 {
		return []interface{}{extras[2]}, nil
	}
	return []interface{}{extras[3]}, nil
}

// TODO(@kiendn): add function that do specific things such as converting numbers from types to types, etc.
