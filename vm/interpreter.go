package vm

import (
	"github.com/kardiachain/go-kardia/params"
)

// Config are the configuration options for the Interpreter
type Config struct {
	// TODO(huny@): Fill this
}

// Interpreter is used to run Kardia based contracts and will utilise the
// passed environment to query external sources for state information.
// The Interpreter will run the byte code VM based on the passed
// configuration.
type Interpreter struct {
	kvm      *KVM
	cfg      Config
	gasTable params.GasTable
	intPool  *intPool

	readOnly   bool   // Whether to throw on stateful modifications
	returnData []byte // Last CALL's return data for subsequent reuse
}

// NewInterpreter returns a new instance of the Interpreter.
func NewInterpreter(kvm *KVM, cfg Config) *Interpreter {
	return &Interpreter{
		kvm:      kvm,
		cfg:      cfg,
		gasTable: kvm.ChainConfig().GasTable(kvm.BlockHeight),
	}
}
