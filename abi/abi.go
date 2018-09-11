package abi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
)

// The ABI holds information about a contract's context and available
// invokable methods. It will allow you to type check function calls and
// packs data accordingly.
type ABI struct {
	Constructor Method
	Methods     map[string]Method
	Events      map[string]Event
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
		Type      string
		Name      string
		Constant  bool
		Anonymous bool
		Inputs    []Argument
		Outputs   []Argument
	}

	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	abi.Methods = make(map[string]Method)
	abi.Events = make(map[string]Event)
	for _, field := range fields {
		switch field.Type {
		case "constructor":
			abi.Constructor = Method{
				Inputs: field.Inputs,
			}
		// empty defaults to function according to the abi spec
		case "function", "":
			abi.Methods[field.Name] = Method{
				Name:    field.Name,
				Const:   field.Constant,
				Inputs:  field.Inputs,
				Outputs: field.Outputs,
			}
		case "event":
			abi.Events[field.Name] = Event{
				Name:      field.Name,
				Anonymous: field.Anonymous,
				Inputs:    field.Inputs,
			}
		}
	}

	return nil
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
	return append(method.Id(), arguments...), nil
}

// Unpack output in v according to the abi specification
func (abi ABI) Unpack(v interface{}, name string, output []byte) (err error) {
	if len(output) == 0 {
		return fmt.Errorf("abi: unmarshalling empty output")
	}
	// since there can't be naming collisions with contracts and events,
	// we need to decide whether we're calling a method or an event
	if method, ok := abi.Methods[name]; ok {
		if len(output)%32 != 0 {
			return fmt.Errorf("abi: improperly formatted output")
		}
		return method.Outputs.Unpack(v, output)
	} else if event, ok := abi.Events[name]; ok {
		return event.Inputs.Unpack(v, output)
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
	for _, method := range abi.Methods {
		if bytes.Equal(method.Id(), sigdata[:4]) {
			return &method, nil
		}
	}
	return nil, fmt.Errorf("no method with id: %#x", sigdata[:4])
}

//==============================================================================
// Method
//==============================================================================
// Method represents a callable given a `Name` and whether the method is a constant.
// If the method is `Const` no transaction needs to be created for this
// particular Method call. It can easily be simulated using a local VM.
// For example a `Balance()` method only needs to retrieve something
// from the storage and therefor requires no Tx to be send to the
// network. A method such as `Transact` does require a Tx and thus will
// be flagged `true`.
// Input specifies the required input parameters for this gives method.
type Method struct {
	Name    string
	Const   bool
	Inputs  Arguments
	Outputs Arguments
}

// Sig returns the methods string signature according to the ABI spec.
//
// Example
//
//     function foo(uint32 a, int b)    =    "foo(uint32,int256)"
//
// Please note that "int" is substitute for its canonical representation "int256"
func (method Method) Sig() string {
	types := make([]string, len(method.Inputs))
	for i, input := range method.Inputs {
		types[i] = input.Type.String()
	}
	return fmt.Sprintf("%v(%v)", method.Name, strings.Join(types, ","))
}

func (method Method) String() string {
	inputs := make([]string, len(method.Inputs))
	for i, input := range method.Inputs {
		inputs[i] = fmt.Sprintf("%v %v", input.Name, input.Type)
	}
	outputs := make([]string, len(method.Outputs))
	for i, output := range method.Outputs {
		if len(output.Name) > 0 {
			outputs[i] = fmt.Sprintf("%v ", output.Name)
		}
		outputs[i] += output.Type.String()
	}
	constant := ""
	if method.Const {
		constant = "constant "
	}
	return fmt.Sprintf("function %v(%v) %sreturns(%v)", method.Name, strings.Join(inputs, ", "), constant, strings.Join(outputs, ", "))
}

func (method Method) Id() []byte {
	return crypto.Keccak256([]byte(method.Sig()))[:4]
}

//==============================================================================
// Event
//==============================================================================
// Event is an event potentially triggered by the KVM's LOG mechanism. The Event
// holds type information (inputs) about the yielded output. Anonymous events
// don't get the signature canonical representation as the first LOG topic.
type Event struct {
	Name      string
	Anonymous bool
	Inputs    Arguments
}

func (e Event) String() string {
	inputs := make([]string, len(e.Inputs))
	for i, input := range e.Inputs {
		inputs[i] = fmt.Sprintf("%v %v", input.Name, input.Type)
		if input.Indexed {
			inputs[i] = fmt.Sprintf("%v indexed %v", input.Name, input.Type)
		}
	}
	return fmt.Sprintf("e %v(%v)", e.Name, strings.Join(inputs, ", "))
}

// Id returns the canonical representation of the event's signature used by the
// abi definition to identify event names and types.
func (e Event) Id() common.Hash {
	types := make([]string, len(e.Inputs))
	i := 0
	for _, input := range e.Inputs {
		types[i] = input.Type.String()
		i++
	}
	return common.BytesToHash(crypto.Keccak256([]byte(fmt.Sprintf("%v(%v)", e.Name, strings.Join(types, ",")))))
}
