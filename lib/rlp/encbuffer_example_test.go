// From go-ethereum/rlp/encbuffer_example_test.go

package rlp_test

import (
	"bytes"
	"fmt"

	"github.com/kardiachain/go-kardia/lib/rlp"
)

func ExampleEncoderBuffer() {
	var w bytes.Buffer

	// Encode [4, [5, 6]] to w.
	buf := rlp.NewEncoderBuffer(&w)
	l1 := buf.List()
	buf.WriteUint64(4)
	l2 := buf.List()
	buf.WriteUint64(5)
	buf.WriteUint64(6)
	buf.ListEnd(l2)
	buf.ListEnd(l1)

	if err := buf.Flush(); err != nil {
		panic(err)
	}
	fmt.Printf("%X\n", w.Bytes())
	// Output:
	// C404C20506
}
