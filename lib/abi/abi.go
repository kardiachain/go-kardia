// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package abi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
)

// The ABI holds information about a contract's context and available
// invokable methods. It will allow you to type check function calls and
// packs data accordingly.
type ABI struct {
	Constructor Method
	Methods     map[string]Method
	Events      map[string]Event

	// Additional "special" functions
	// It's separated from the original default fallback. Each contract
	// can only define one fallback and receive function.
	Fallback Method
	Receive  Method
}

// JSON returns a parsed ABI interface and error if it failed.
func JSON(reader io.Reader) (ABI, error) {
	dec := json.NewDecoder(reader)

	var abi ABI
	if err := dec.Decode(&abi); err != nil {
		return ABI{}, err
	}

	return abi, nil
}

// UnmarshalJSON implements json.Unmarshaler interface
func (abi *ABI) UnmarshalJSON(data []byte) error {
	var fields []struct {
		Type     string
		Name     string
		Constant bool // True if function is either pure or view

		// Event relevant indicator represents the event is
		// declared as anonymous.
		Anonymous bool

		Inputs  []Argument
		Outputs []Argument

		// Status indicator which can be: "pure", "view",
		// "nonpayable" or "payable".
		StateMutability string
		Payable         bool // True if function is payable
	}

	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	abi.Methods = make(map[string]Method)
	abi.Events = make(map[string]Event)
	for _, field := range fields {
		switch field.Type {
		case "constructor":
			abi.Constructor = NewMethod("", "", Constructor, field.StateMutability, field.Constant, field.Payable, field.Inputs, nil)
		case "function":
			name := abi.overloadedMethodName(field.Name)
			abi.Methods[name] = NewMethod(name, field.Name, Function, field.StateMutability, field.Constant, field.Payable, field.Inputs, field.Outputs)
		case "fallback":
			if abi.HasFallback() {
				return errors.New("only single fallback is allowed")
			}
			abi.Fallback = NewMethod("", "", Fallback, field.StateMutability, field.Constant, field.Payable, nil, nil)
		case "receive":
			if abi.HasReceive() {
				return errors.New("only single receive is allowed")
			}
			if field.StateMutability != "payable" {
				return errors.New("the statemutability of receive can only be payable")
			}
			abi.Receive = NewMethod("", "", Receive, field.StateMutability, field.Constant, field.Payable, nil, nil)
		case "event":
			name := abi.overloadedEventName(field.Name)
			abi.Events[name] = NewEvent(name, field.Name, field.Anonymous, field.Inputs)
		default:
			return fmt.Errorf("abi: could not recognize type %v of field %v", field.Type, field.Name)
		}
	}

	return nil
}

// overloadedMethodName returns the next available name for a given function.
// Needed since solidity allows for function overload.
//
// e.g. if the abi contains Methods send, send1
// overloadedMethodName would return send2 for input send.
func (abi *ABI) overloadedMethodName(rawName string) string {
	name := rawName
	_, ok := abi.Methods[name]
	for idx := 0; ok; idx++ {
		name = fmt.Sprintf("%s%d", rawName, idx)
		_, ok = abi.Methods[name]
	}
	return name
}

// overloadedEventName returns the next available name for a given event.
// Needed since solidity allows for event overload.
//
// e.g. if the abi contains events received, received1
// overloadedEventName would return received2 for input received.
func (abi *ABI) overloadedEventName(rawName string) string {
	name := rawName
	_, ok := abi.Events[name]
	for idx := 0; ok; idx++ {
		name = fmt.Sprintf("%s%d", rawName, idx)
		_, ok = abi.Events[name]
	}
	return name
}

// Pack the given method name to conform the ABI. Method call's data
// will consist of method_id, args0, arg1, ... argN. Method id consists
// of 4 bytes and arguments are all 32 bytes.
// Method ids are created from the first 4 bytes of the hash of the
// methods string signature. (signature = baz(uint32,string32))
func (abi ABI) Pack(name string, args ...interface{}) ([]byte, error) {
	// Fetch the ABI of the requested method
	if name == "" {
		// constructor
		arguments, err := abi.Constructor.Inputs.Pack(args...)
		if err != nil {
			return nil, err
		}
		return arguments, nil
	}
	method, exist := abi.Methods[name]
	if !exist {
		return nil, fmt.Errorf("method '%s' not found", name)
	}
	arguments, err := method.Inputs.Pack(args...)
	if err != nil {
		return nil, err
	}
	// Pack up the method ID too if not a constructor and return
	return append(method.ID, arguments...), nil
}

// Unpack output in v according to the abi specification
func (abi ABI) Unpack(v interface{}, name string, output []byte) (err error) {
	// since there can't be naming collisions with contracts and events,
	// we need to decide whether we're calling a method or an event
	if method, ok := abi.Methods[name]; ok {
		if len(output)%32 != 0 {
			return fmt.Errorf("abi: improperly formatted output: %s - Bytes: [%+v]", string(output), output)
		}
		return method.Outputs.Unpack(v, output)
	}
	if event, ok := abi.Events[name]; ok {
		return event.Inputs.Unpack(v, output)
	}
	return fmt.Errorf("abi: could not locate named method or event")
}

// UnpackIntoMap unpacks a log into the provided map[string]interface{}
func (abi ABI) UnpackIntoMap(v map[string]interface{}, name string, data []byte) (err error) {
	// since there can't be naming collisions with contracts and events,
	// we need to decide whether we're calling a method or an event
	if method, ok := abi.Methods[name]; ok {
		if len(data)%32 != 0 {
			return fmt.Errorf("abi: improperly formatted output")
		}
		return method.Outputs.UnpackIntoMap(v, data)
	}
	if event, ok := abi.Events[name]; ok {
		return event.Inputs.UnpackIntoMap(v, data)
	}
	return fmt.Errorf("abi: could not locate named method or event")
}

// UnpackInput unpacks inputs of Method to v according to the abi specification
func (abi ABI) UnpackInput(v interface{}, name string, output []byte) (err error) {
	if len(output) == 0 {
		return fmt.Errorf("abi: unmarshalling empty output")
	}
	// since there can't be naming collisions with contracts and events,
	// we need to decide whether we're calling a method or an event
	if method, ok := abi.Methods[name]; ok {
		if len(output)%32 != 0 {
			return fmt.Errorf("abi: improperly formatted output")
		}
		return method.Inputs.Unpack(v, output)
	}
	return fmt.Errorf("abi: could not locate named method or event")
}

// MethodById looks up a method by the 4-byte id
// returns nil if none found
func (abi *ABI) MethodById(sigdata []byte) (*Method, error) {
	if len(sigdata) < 4 {
		return nil, fmt.Errorf("data too short (%d bytes) for abi method lookup", len(sigdata))
	}
	for _, method := range abi.Methods {
		if bytes.Equal(method.ID, sigdata[:4]) {
			return &method, nil
		}
	}
	return nil, fmt.Errorf("no method with id: %#x", sigdata[:4])
}

// EventByID looks an event up by its topic hash in the
// ABI and returns nil if none found.
func (abi *ABI) EventByID(topic common.Hash) (*Event, error) {
	for _, event := range abi.Events {
		if bytes.Equal(event.ID.Bytes(), topic.Bytes()) {
			return &event, nil
		}
	}
	return nil, fmt.Errorf("no event with id: %#x", topic.Hex())
}

// HasFallback returns an indicator whether a fallback function is included.
func (abi *ABI) HasFallback() bool {
	return abi.Fallback.Type == Fallback
}

// HasReceive returns an indicator whether a receive function is included.
func (abi *ABI) HasReceive() bool {
	return abi.Receive.Type == Receive
}

// revertSelector is a special function selector for revert reason unpacking.
var revertSelector = crypto.Keccak256([]byte("Error(string)"))[:4]

// UnpackRevert resolves the abi-encoded revert reason.
// the provided revert reason is abi-encoded as if it were a call to a function
// `Error(string)`. So it's a special tool for it.
func UnpackRevert(data []byte) (string, error) {
	if len(data) < 4 {
		return "", errors.New("invalid data for unpacking")
	}
	if !bytes.Equal(data[:4], revertSelector) {
		return "", errors.New("invalid data for unpacking")
	}
	var reason string
	typ, _ := NewType("string", "", nil)
	if err := (Arguments{{Type: typ}}).Unpack(&reason, data[4:]); err != nil {
		return "", err
	}
	return reason, nil
}

//==============================================================================
// Method
//==============================================================================

// FunctionType represents different types of functions a contract might have.
type FunctionType int

const (
	// Constructor represents the constructor of the contract.
	// The constructor function is called while deploying a contract.
	Constructor FunctionType = iota
	// Fallback represents the fallback function.
	// This function is executed if no other function matches the given function
	// signature and no receive function is specified.
	Fallback
	// Receive represents the receive function.
	// This function is executed on plain Ether transfers.
	Receive
	// Function represents a normal function.
	Function
)

// Method represents a callable given a `Name` and whether the method is a constant.
// If the method is `Const` no transaction needs to be created for this
// particular Method call. It can easily be simulated using a local VM.
// For example a `Balance()` method only needs to retrieve something
// from the storage and therefore requires no Tx to be send to the
// network. A method such as `Transact` does require a Tx and thus will
// be flagged `true`.
// Input specifies the required input parameters for this gives method.
type Method struct {
	// Name is the method name used for internal representation. It's derived from
	// the raw name and a suffix will be added in the case of a function overload.
	//
	// e.g.
	// There are two functions have same name:
	// * foo(int,int)
	// * foo(uint,uint)
	// The method name of the first one will be resolved as foo while the second one
	// will be resolved as foo0.
	Name    string
	Inputs  Arguments
	Outputs Arguments

	RawName string // RawName is the raw method name parsed from ABI

	// Type indicates whether the method is a special fallback
	Type FunctionType

	// StateMutability indicates the mutability state of method,
	// the default value is nonpayable. It can be empty if the abi
	// is generated by legacy compiler.
	StateMutability string

	Constant bool
	Payable  bool
	str      string
	// Sig returns the methods string signature according to the ABI spec.
	// e.g.		function foo(uint32 a, int b) = "foo(uint32,int256)"
	// Please note that "int" is substitute for its canonical representation "int256"
	Sig string
	// ID returns the canonical representation of the method's signature used by the
	// abi definition to identify method names and types.
	ID []byte
}

// NewMethod creates a new Method.
// A method should always be created using NewMethod.
// It also precomputes the sig representation and the string representation
// of the method.
func NewMethod(name string, rawName string, funType FunctionType, mutability string, isConst, isPayable bool, inputs Arguments, outputs Arguments) Method {
	var (
		types       = make([]string, len(inputs))
		inputNames  = make([]string, len(inputs))
		outputNames = make([]string, len(outputs))
	)
	for i, input := range inputs {
		inputNames[i] = fmt.Sprintf("%v %v", input.Type, input.Name)
		types[i] = input.Type.String()
	}
	for i, output := range outputs {
		outputNames[i] = output.Type.String()
		if len(output.Name) > 0 {
			outputNames[i] += fmt.Sprintf(" %v", output.Name)
		}
	}
	// calculate the signature and method id. Note only function
	// has meaningful signature and id.
	var (
		sig string
		id  []byte
	)
	if funType == Function {
		sig = fmt.Sprintf("%v(%v)", rawName, strings.Join(types, ","))
		id = crypto.Keccak256([]byte(sig))[:4]
	}
	// Extract meaningful state mutability of solidity method.
	// If it's default value, never print it.
	state := mutability
	if state == "nonpayable" {
		state = ""
	}
	if state != "" {
		state = state + " "
	}
	identity := fmt.Sprintf("function %v", rawName)
	if funType == Fallback {
		identity = "fallback"
	} else if funType == Receive {
		identity = "receive"
	} else if funType == Constructor {
		identity = "constructor"
	}

	str := fmt.Sprintf("%v(%v) %sreturns(%v)", identity, strings.Join(inputNames, ", "), state, strings.Join(outputNames, ", "))

	return Method{
		Name:            name,
		RawName:         rawName,
		Type:            funType,
		StateMutability: mutability,
		Constant:        isConst,
		Payable:         isPayable,
		Inputs:          inputs,
		Outputs:         outputs,
		str:             str,
		Sig:             sig,
		ID:              id,
	}
}

func (method Method) String() string {
	return method.str
}

// IsConstant returns the indicator whether the method is read-only.
func (method Method) IsConstant() bool {
	return method.StateMutability == "view" || method.StateMutability == "pure" || method.Constant
}

// IsPayable returns the indicator whether the method can process
// plain ether transfers.
func (method Method) IsPayable() bool {
	return method.StateMutability == "payable" || method.Payable
}

//==============================================================================
// Event
//==============================================================================
// Event is an event potentially triggered by the KVM's LOG mechanism. The Event
// holds type information (inputs) about the yielded output. Anonymous events
// don't get the signature canonical representation as the first LOG topic.
type Event struct {
	// Name is the event name used for internal representation. It's derived from
	// the raw name and a suffix will be added in the case of a event overload.
	//
	// e.g.
	// There are two events have same name:
	// * foo(int,int)
	// * foo(uint,uint)
	// The event name of the first one wll be resolved as foo while the second one
	// will be resolved as foo0.
	Name      string
	Anonymous bool
	Inputs    Arguments
	// RawName is the raw event name parsed from ABI.
	RawName string
	str     string
	// Sig contains the string signature according to the ABI spec.
	// e.g.	 event foo(uint32 a, int b) = "foo(uint32,int256)"
	// Please note that "int" is substitute for its canonical representation "int256"
	Sig string
	// ID returns the canonical representation of the event's signature used by the
	// abi definition to identify event names and types.
	ID common.Hash
}

// NewEvent creates a new Event.
// It sanitizes the input arguments to remove unnamed arguments.
// It also precomputes the id, signature and string representation
// of the event.
func NewEvent(name, rawName string, anonymous bool, inputs Arguments) Event {
	// sanitize inputs to remove inputs without names
	// and precompute string and sig representation.
	names := make([]string, len(inputs))
	types := make([]string, len(inputs))
	for i, input := range inputs {
		if input.Name == "" {
			inputs[i] = Argument{
				Name:    fmt.Sprintf("arg%d", i),
				Indexed: input.Indexed,
				Type:    input.Type,
			}
		} else {
			inputs[i] = input
		}
		// string representation
		names[i] = fmt.Sprintf("%v %v", input.Type, inputs[i].Name)
		if input.Indexed {
			names[i] = fmt.Sprintf("%v indexed %v", input.Type, inputs[i].Name)
		}
		// sig representation
		types[i] = input.Type.String()
	}
	str := fmt.Sprintf("event %v(%v)", rawName, strings.Join(names, ", "))
	sig := fmt.Sprintf("%v(%v)", rawName, strings.Join(types, ","))
	id := common.BytesToHash(crypto.Keccak256([]byte(sig)))

	return Event{
		Name:      name,
		RawName:   rawName,
		Anonymous: anonymous,
		Inputs:    inputs,
		str:       str,
		Sig:       sig,
		ID:        id,
	}
}

func (e Event) String() string {
	return e.str
}
