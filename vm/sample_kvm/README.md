# Sample KVM

### Description
Sample KVM provides an easy way to test runtime bytecodes on Kardia Virtual Machine (KVM).

### Instructions
1. Write a smart contract in Solidity
2. Compile the smart contract to generate *runtime* bytecodes via the compiler of choice e.g. remix, solc, truffle etc.
3. Get the Application Binary Interface (ABI) of the smart contract
4. Create a unit test case in `sample_kvm_test.go` which calls the smart contract via the simulated KVM
5. Run `go test` and make sure all tests pass

### Note
1. When compiling, the compiler will generate both bytecode and runtime_bytecode for the smart contract. Since this tool is a standalone, it will use the runtime bytecode only. Inputing normal bytecode will cause KVM to panic.
2. Compiler with different versions and options will generate different bytecodes. Make sure the original smart contract content with compiler version and options are clearly denoted in the unit tests.

