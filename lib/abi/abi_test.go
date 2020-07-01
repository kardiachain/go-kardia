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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jsondata = `
[
	{ "type" : "function", "name" : "balance", "constant" : true },
	{ "type" : "function", "name" : "send", "constant" : false, "inputs" : [ { "name" : "amount", "type" : "uint256" } ] }
]`

const jsondata2 = `
[
	{ "type" : "function", "name" : "balance", "constant" : true },
	{ "type" : "function", "name" : "send", "constant" : false, "inputs" : [ { "name" : "amount", "type" : "uint256" } ] },
	{ "type" : "function", "name" : "test", "constant" : false, "inputs" : [ { "name" : "number", "type" : "uint32" } ] },
	{ "type" : "function", "name" : "string", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "string" } ] },
	{ "type" : "function", "name" : "bool", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "bool" } ] },
	{ "type" : "function", "name" : "address", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "address" } ] },
	{ "type" : "function", "name" : "uint64[2]", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint64[2]" } ] },
	{ "type" : "function", "name" : "uint64[]", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint64[]" } ] },
	{ "type" : "function", "name" : "foo", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint32" } ] },
	{ "type" : "function", "name" : "bar", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint32" }, { "name" : "string", "type" : "uint16" } ] },
	{ "type" : "function", "name" : "slice", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint32[2]" } ] },
	{ "type" : "function", "name" : "slice256", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint256[2]" } ] },
	{ "type" : "function", "name" : "sliceAddress", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "address[]" } ] },
	{ "type" : "function", "name" : "sliceMultiAddress", "constant" : false, "inputs" : [ { "name" : "a", "type" : "address[]" }, { "name" : "b", "type" : "address[]" } ] }
]`

func TestReader(t *testing.T) {
	Uint256, _ := NewType("uint256")
	exp := ABI{
		Methods: map[string]Method{
			"balance": {
				"balance", true, nil, nil,
			},
			"send": {
				"send", false, []Argument{
					{"amount", Uint256, false},
				}, nil,
			},
		},
	}

	abi, err := JSON(strings.NewReader(jsondata))
	if err != nil {
		t.Error(err)
	}

	// deep equal fails for some reason
	for name, expM := range exp.Methods {
		gotM, exist := abi.Methods[name]
		if !exist {
			t.Errorf("Missing expected method %v", name)
		}
		if !reflect.DeepEqual(gotM, expM) {
			t.Errorf("\nGot abi method: \n%v\ndoes not match expected method\n%v", gotM, expM)
		}
	}

	for name, gotM := range abi.Methods {
		expM, exist := exp.Methods[name]
		if !exist {
			t.Errorf("Found extra method %v", name)
		}
		if !reflect.DeepEqual(gotM, expM) {
			t.Errorf("\nGot abi method: \n%v\ndoes not match expected method\n%v", gotM, expM)
		}
	}
}

func TestTestNumbers(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if _, err := abi.Pack("balance"); err != nil {
		t.Error(err)
	}

	if _, err := abi.Pack("balance", 1); err == nil {
		t.Error("expected error for balance(1)")
	}

	if _, err := abi.Pack("doesntexist", nil); err == nil {
		t.Errorf("doesntexist shouldn't exist")
	}

	if _, err := abi.Pack("doesntexist", 1); err == nil {
		t.Errorf("doesntexist(1) shouldn't exist")
	}

	if _, err := abi.Pack("send", big.NewInt(1000)); err != nil {
		t.Error(err)
	}

	i := new(int)
	*i = 1000
	if _, err := abi.Pack("send", i); err == nil {
		t.Errorf("expected send( ptr ) to throw, requires *big.Int instead of *int")
	}

	if _, err := abi.Pack("test", uint32(1000)); err != nil {
		t.Error(err)
	}
}

func TestTestString(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if _, err := abi.Pack("string", "hello world"); err != nil {
		t.Error(err)
	}
}

func TestTestBool(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if _, err := abi.Pack("bool", true); err != nil {
		t.Error(err)
	}
}

func TestTestSlice(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	slice := make([]uint64, 2)
	if _, err := abi.Pack("uint64[2]", slice); err != nil {
		t.Error(err)
	}

	if _, err := abi.Pack("uint64[]", slice); err != nil {
		t.Error(err)
	}
}

func TestMethodSignature(t *testing.T) {
	String, _ := NewType("string")
	m := Method{"foo", false, []Argument{{"bar", String, false}, {"baz", String, false}}, nil}
	exp := "foo(string,string)"
	if m.Sig() != exp {
		t.Error("signature mismatch", exp, "!=", m.Sig())
	}

	idexp := crypto.Keccak256([]byte(exp))[:4]
	if !bytes.Equal(m.Id(), idexp) {
		t.Errorf("expected ids to match %x != %x", m.Id(), idexp)
	}

	uintt, _ := NewType("uint256")
	m = Method{"foo", false, []Argument{{"bar", uintt, false}}, nil}
	exp = "foo(uint256)"
	if m.Sig() != exp {
		t.Error("signature mismatch", exp, "!=", m.Sig())
	}
}

func TestMultiPack(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	sig := crypto.Keccak256([]byte("bar(uint32,uint16)"))[:4]
	sig = append(sig, make([]byte, 64)...)
	sig[35] = 10
	sig[67] = 11

	packed, err := abi.Pack("bar", uint32(10), uint16(11))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}
}

func ExampleJSON() {
	const definition = `[{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"isBar","outputs":[{"name":"","type":"bool"}],"type":"function"}]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		log.Fatalln(err)
	}
	out, err := abi.Pack("isBar", common.HexToAddress("01"))
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("%x\n", out)
	// Output:
	// 1f2c40920000000000000000000000000000000000000000000000000000000000000001
}

func TestInputVariableInputLength(t *testing.T) {
	const definition = `[
	{ "type" : "function", "name" : "strOne", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" } ] },
	{ "type" : "function", "name" : "bytesOne", "constant" : true, "inputs" : [ { "name" : "str", "type" : "bytes" } ] },
	{ "type" : "function", "name" : "strTwo", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "str1", "type" : "string" } ] }
	]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	// test one string
	strin := "hello world"
	strpack, err := abi.Pack("strOne", strin)
	if err != nil {
		t.Error(err)
	}

	offset := make([]byte, 32)
	offset[31] = 32
	length := make([]byte, 32)
	length[31] = byte(len(strin))
	value := common.RightPadBytes([]byte(strin), 32)
	exp := append(offset, append(length, value...)...)

	// ignore first 4 bytes of the output. This is the function identifier
	strpack = strpack[4:]
	if !bytes.Equal(strpack, exp) {
		t.Errorf("expected %x, got %x\n", exp, strpack)
	}

	// test one bytes
	btspack, err := abi.Pack("bytesOne", []byte(strin))
	if err != nil {
		t.Error(err)
	}
	// ignore first 4 bytes of the output. This is the function identifier
	btspack = btspack[4:]
	if !bytes.Equal(btspack, exp) {
		t.Errorf("expected %x, got %x\n", exp, btspack)
	}

	//  test two strings
	str1 := "hello"
	str2 := "world"
	str2pack, err := abi.Pack("strTwo", str1, str2)
	if err != nil {
		t.Error(err)
	}

	offset1 := make([]byte, 32)
	offset1[31] = 64
	length1 := make([]byte, 32)
	length1[31] = byte(len(str1))
	value1 := common.RightPadBytes([]byte(str1), 32)

	offset2 := make([]byte, 32)
	offset2[31] = 128
	length2 := make([]byte, 32)
	length2[31] = byte(len(str2))
	value2 := common.RightPadBytes([]byte(str2), 32)

	exp2 := append(offset1, offset2...)
	exp2 = append(exp2, append(length1, value1...)...)
	exp2 = append(exp2, append(length2, value2...)...)

	// ignore first 4 bytes of the output. This is the function identifier
	str2pack = str2pack[4:]
	if !bytes.Equal(str2pack, exp2) {
		t.Errorf("expected %x, got %x\n", exp, str2pack)
	}

	// test two strings, first > 32, second < 32
	str1 = strings.Repeat("a", 33)
	str2pack, err = abi.Pack("strTwo", str1, str2)
	if err != nil {
		t.Error(err)
	}

	offset1 = make([]byte, 32)
	offset1[31] = 64
	length1 = make([]byte, 32)
	length1[31] = byte(len(str1))
	value1 = common.RightPadBytes([]byte(str1), 64)
	offset2[31] = 160

	exp2 = append(offset1, offset2...)
	exp2 = append(exp2, append(length1, value1...)...)
	exp2 = append(exp2, append(length2, value2...)...)

	// ignore first 4 bytes of the output. This is the function identifier
	str2pack = str2pack[4:]
	if !bytes.Equal(str2pack, exp2) {
		t.Errorf("expected %x, got %x\n", exp, str2pack)
	}

	// test two strings, first > 32, second >32
	str1 = strings.Repeat("a", 33)
	str2 = strings.Repeat("a", 33)
	str2pack, err = abi.Pack("strTwo", str1, str2)
	if err != nil {
		t.Error(err)
	}

	offset1 = make([]byte, 32)
	offset1[31] = 64
	length1 = make([]byte, 32)
	length1[31] = byte(len(str1))
	value1 = common.RightPadBytes([]byte(str1), 64)

	offset2 = make([]byte, 32)
	offset2[31] = 160
	length2 = make([]byte, 32)
	length2[31] = byte(len(str2))
	value2 = common.RightPadBytes([]byte(str2), 64)

	exp2 = append(offset1, offset2...)
	exp2 = append(exp2, append(length1, value1...)...)
	exp2 = append(exp2, append(length2, value2...)...)

	// ignore first 4 bytes of the output. This is the function identifier
	str2pack = str2pack[4:]
	if !bytes.Equal(str2pack, exp2) {
		t.Errorf("expected %x, got %x\n", exp, str2pack)
	}
}

func TestInputFixedArrayAndVariableInputLength(t *testing.T) {
	const definition = `[
	{ "type" : "function", "name" : "fixedArrStr", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr", "type" : "uint256[2]" } ] },
	{ "type" : "function", "name" : "fixedArrBytes", "constant" : true, "inputs" : [ { "name" : "str", "type" : "bytes" }, { "name" : "fixedArr", "type" : "uint256[2]" } ] },
    { "type" : "function", "name" : "mixedArrStr", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr", "type": "uint256[2]" }, { "name" : "dynArr", "type": "uint256[]" } ] },
    { "type" : "function", "name" : "doubleFixedArrStr", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr1", "type": "uint256[2]" }, { "name" : "fixedArr2", "type": "uint256[3]" } ] },
    { "type" : "function", "name" : "multipleMixedArrStr", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr1", "type": "uint256[2]" }, { "name" : "dynArr", "type" : "uint256[]" }, { "name" : "fixedArr2", "type" : "uint256[3]" } ] }
	]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Error(err)
	}

	// test string, fixed array uint256[2]
	strin := "hello world"
	arrin := [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	fixedArrStrPack, err := abi.Pack("fixedArrStr", strin, arrin)
	if err != nil {
		t.Error(err)
	}

	// generate expected output
	offset := make([]byte, 32)
	offset[31] = 96
	length := make([]byte, 32)
	length[31] = byte(len(strin))
	strvalue := common.RightPadBytes([]byte(strin), 32)
	arrinvalue1 := common.LeftPadBytes(arrin[0].Bytes(), 32)
	arrinvalue2 := common.LeftPadBytes(arrin[1].Bytes(), 32)
	exp := append(offset, arrinvalue1...)
	exp = append(exp, arrinvalue2...)
	exp = append(exp, append(length, strvalue...)...)

	// ignore first 4 bytes of the output. This is the function identifier
	fixedArrStrPack = fixedArrStrPack[4:]
	if !bytes.Equal(fixedArrStrPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, fixedArrStrPack)
	}

	// test byte array, fixed array uint256[2]
	bytesin := []byte(strin)
	arrin = [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	fixedArrBytesPack, err := abi.Pack("fixedArrBytes", bytesin, arrin)
	if err != nil {
		t.Error(err)
	}

	// generate expected output
	offset = make([]byte, 32)
	offset[31] = 96
	length = make([]byte, 32)
	length[31] = byte(len(strin))
	strvalue = common.RightPadBytes([]byte(strin), 32)
	arrinvalue1 = common.LeftPadBytes(arrin[0].Bytes(), 32)
	arrinvalue2 = common.LeftPadBytes(arrin[1].Bytes(), 32)
	exp = append(offset, arrinvalue1...)
	exp = append(exp, arrinvalue2...)
	exp = append(exp, append(length, strvalue...)...)

	// ignore first 4 bytes of the output. This is the function identifier
	fixedArrBytesPack = fixedArrBytesPack[4:]
	if !bytes.Equal(fixedArrBytesPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, fixedArrBytesPack)
	}

	// test string, fixed array uint256[2], dynamic array uint256[]
	strin = "hello world"
	fixedarrin := [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	dynarrin := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	mixedArrStrPack, err := abi.Pack("mixedArrStr", strin, fixedarrin, dynarrin)
	if err != nil {
		t.Error(err)
	}

	// generate expected output
	stroffset := make([]byte, 32)
	stroffset[31] = 128
	strlength := make([]byte, 32)
	strlength[31] = byte(len(strin))
	strvalue = common.RightPadBytes([]byte(strin), 32)
	fixedarrinvalue1 := common.LeftPadBytes(fixedarrin[0].Bytes(), 32)
	fixedarrinvalue2 := common.LeftPadBytes(fixedarrin[1].Bytes(), 32)
	dynarroffset := make([]byte, 32)
	dynarroffset[31] = byte(160 + ((len(strin)/32)+1)*32)
	dynarrlength := make([]byte, 32)
	dynarrlength[31] = byte(len(dynarrin))
	dynarrinvalue1 := common.LeftPadBytes(dynarrin[0].Bytes(), 32)
	dynarrinvalue2 := common.LeftPadBytes(dynarrin[1].Bytes(), 32)
	dynarrinvalue3 := common.LeftPadBytes(dynarrin[2].Bytes(), 32)
	exp = append(stroffset, fixedarrinvalue1...)
	exp = append(exp, fixedarrinvalue2...)
	exp = append(exp, dynarroffset...)
	exp = append(exp, append(strlength, strvalue...)...)
	dynarrarg := append(dynarrlength, dynarrinvalue1...)
	dynarrarg = append(dynarrarg, dynarrinvalue2...)
	dynarrarg = append(dynarrarg, dynarrinvalue3...)
	exp = append(exp, dynarrarg...)

	// ignore first 4 bytes of the output. This is the function identifier
	mixedArrStrPack = mixedArrStrPack[4:]
	if !bytes.Equal(mixedArrStrPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, mixedArrStrPack)
	}

	// test string, fixed array uint256[2], fixed array uint256[3]
	strin = "hello world"
	fixedarrin1 := [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	fixedarrin2 := [3]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	doubleFixedArrStrPack, err := abi.Pack("doubleFixedArrStr", strin, fixedarrin1, fixedarrin2)
	if err != nil {
		t.Error(err)
	}

	// generate expected output
	stroffset = make([]byte, 32)
	stroffset[31] = 192
	strlength = make([]byte, 32)
	strlength[31] = byte(len(strin))
	strvalue = common.RightPadBytes([]byte(strin), 32)
	fixedarrin1value1 := common.LeftPadBytes(fixedarrin1[0].Bytes(), 32)
	fixedarrin1value2 := common.LeftPadBytes(fixedarrin1[1].Bytes(), 32)
	fixedarrin2value1 := common.LeftPadBytes(fixedarrin2[0].Bytes(), 32)
	fixedarrin2value2 := common.LeftPadBytes(fixedarrin2[1].Bytes(), 32)
	fixedarrin2value3 := common.LeftPadBytes(fixedarrin2[2].Bytes(), 32)
	exp = append(stroffset, fixedarrin1value1...)
	exp = append(exp, fixedarrin1value2...)
	exp = append(exp, fixedarrin2value1...)
	exp = append(exp, fixedarrin2value2...)
	exp = append(exp, fixedarrin2value3...)
	exp = append(exp, append(strlength, strvalue...)...)

	// ignore first 4 bytes of the output. This is the function identifier
	doubleFixedArrStrPack = doubleFixedArrStrPack[4:]
	if !bytes.Equal(doubleFixedArrStrPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, doubleFixedArrStrPack)
	}

	// test string, fixed array uint256[2], dynamic array uint256[], fixed array uint256[3]
	strin = "hello world"
	fixedarrin1 = [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	dynarrin = []*big.Int{big.NewInt(1), big.NewInt(2)}
	fixedarrin2 = [3]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	multipleMixedArrStrPack, err := abi.Pack("multipleMixedArrStr", strin, fixedarrin1, dynarrin, fixedarrin2)
	if err != nil {
		t.Error(err)
	}

	// generate expected output
	stroffset = make([]byte, 32)
	stroffset[31] = 224
	strlength = make([]byte, 32)
	strlength[31] = byte(len(strin))
	strvalue = common.RightPadBytes([]byte(strin), 32)
	fixedarrin1value1 = common.LeftPadBytes(fixedarrin1[0].Bytes(), 32)
	fixedarrin1value2 = common.LeftPadBytes(fixedarrin1[1].Bytes(), 32)
	dynarroffset = U256(big.NewInt(int64(256 + ((len(strin)/32)+1)*32)))
	dynarrlength = make([]byte, 32)
	dynarrlength[31] = byte(len(dynarrin))
	dynarrinvalue1 = common.LeftPadBytes(dynarrin[0].Bytes(), 32)
	dynarrinvalue2 = common.LeftPadBytes(dynarrin[1].Bytes(), 32)
	fixedarrin2value1 = common.LeftPadBytes(fixedarrin2[0].Bytes(), 32)
	fixedarrin2value2 = common.LeftPadBytes(fixedarrin2[1].Bytes(), 32)
	fixedarrin2value3 = common.LeftPadBytes(fixedarrin2[2].Bytes(), 32)
	exp = append(stroffset, fixedarrin1value1...)
	exp = append(exp, fixedarrin1value2...)
	exp = append(exp, dynarroffset...)
	exp = append(exp, fixedarrin2value1...)
	exp = append(exp, fixedarrin2value2...)
	exp = append(exp, fixedarrin2value3...)
	exp = append(exp, append(strlength, strvalue...)...)
	dynarrarg = append(dynarrlength, dynarrinvalue1...)
	dynarrarg = append(dynarrarg, dynarrinvalue2...)
	exp = append(exp, dynarrarg...)

	// ignore first 4 bytes of the output. This is the function identifier
	multipleMixedArrStrPack = multipleMixedArrStrPack[4:]
	if !bytes.Equal(multipleMixedArrStrPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, multipleMixedArrStrPack)
	}
}

func TestDefaultFunctionParsing(t *testing.T) {
	const definition = `[{ "name" : "balance" }]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := abi.Methods["balance"]; !ok {
		t.Error("expected 'balance' to be present")
	}
}

func TestBareEvents(t *testing.T) {
	const definition = `[
	{ "type" : "event", "name" : "balance" },
	{ "type" : "event", "name" : "anon", "anonymous" : true},
	{ "type" : "event", "name" : "args", "inputs" : [{ "indexed":false, "name":"arg0", "type":"uint256" }, { "indexed":true, "name":"arg1", "type":"address" }] }
	]`

	arg0, _ := NewType("uint256")
	arg1, _ := NewType("address")

	expectedEvents := map[string]struct {
		Anonymous bool
		Args      []Argument
	}{
		"balance": {false, nil},
		"anon":    {true, nil},
		"args": {false, []Argument{
			{Name: "arg0", Type: arg0, Indexed: false},
			{Name: "arg1", Type: arg1, Indexed: true},
		}},
	}

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	if len(abi.Events) != len(expectedEvents) {
		t.Fatalf("invalid number of events after parsing, want %d, got %d", len(expectedEvents), len(abi.Events))
	}

	for name, exp := range expectedEvents {
		got, ok := abi.Events[name]
		if !ok {
			t.Errorf("could not found event %s", name)
			continue
		}
		if got.Anonymous != exp.Anonymous {
			t.Errorf("invalid anonymous indication for event %s, want %v, got %v", name, exp.Anonymous, got.Anonymous)
		}
		if len(got.Inputs) != len(exp.Args) {
			t.Errorf("invalid number of args, want %d, got %d", len(exp.Args), len(got.Inputs))
			continue
		}
		for i, arg := range exp.Args {
			if arg.Name != got.Inputs[i].Name {
				t.Errorf("events[%s].Input[%d] has an invalid name, want %s, got %s", name, i, arg.Name, got.Inputs[i].Name)
			}
			if arg.Indexed != got.Inputs[i].Indexed {
				t.Errorf("events[%s].Input[%d] has an invalid indexed indication, want %v, got %v", name, i, arg.Indexed, got.Inputs[i].Indexed)
			}
			if arg.Type.T != got.Inputs[i].Type.T {
				t.Errorf("events[%s].Input[%d] has an invalid type, want %x, got %x", name, i, arg.Type.T, got.Inputs[i].Type.T)
			}
		}
	}
}

// TestUnpackEvent is based on this contract:
//    contract T {
//      event received(address sender, uint amount, bytes memo);
//      event receivedAddr(address sender);
//      function receive(bytes memo) external payable {
//        received(msg.sender, msg.value, memo);
//        receivedAddr(msg.sender);
//      }
//    }
// When receive("X") is called with sender 0x00... and value 1, it produces this tx receipt:
//   receipt{status=1 cgas=23949 bloom=00000000004000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000040200000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000 logs=[log: b6818c8064f645cd82d99b59a1a267d6d61117ef [75fd880d39c1daf53b6547ab6cb59451fc6452d27caa90e5b6649dd8293b9eed] 000000000000000000000000376c47978271565f56deb45495afa69e59c16ab200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000158 9ae378b6d4409eada347a5dc0c180f186cb62dc68fcc0f043425eb917335aa28 0 95d429d309bb9d753954195fe2d69bd140b4ae731b9b5b605c34323de162cf00 0]}
func TestUnpackEvent(t *testing.T) {
	const abiJSON = `[{"constant":false,"inputs":[{"name":"memo","type":"bytes"}],"name":"receive","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},{"anonymous":false,"inputs":[{"indexed":false,"name":"sender","type":"address"},{"indexed":false,"name":"amount","type":"uint256"},{"indexed":false,"name":"memo","type":"bytes"}],"name":"received","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"name":"sender","type":"address"}],"name":"receivedAddr","type":"event"}]`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}

	const hexdata = `000000000000000000000000376c47978271565f56deb45495afa69e59c16ab200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000158`
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		t.Fatal(err)
	}
	if len(data)%32 == 0 {
		t.Errorf("len(data) is %d, want a non-multiple of 32", len(data))
	}

	type ReceivedEvent struct {
		Address common.Address
		Amount  *big.Int
		Memo    []byte
	}
	var ev ReceivedEvent

	err = abi.Unpack(&ev, "received", data)
	if err != nil {
		t.Error(err)
	} else {
		t.Logf("len(data): %d; received event: %+v", len(data), ev)
	}

	type ReceivedAddrEvent struct {
		Address common.Address
	}
	var receivedAddrEv ReceivedAddrEvent
	err = abi.Unpack(&receivedAddrEv, "receivedAddr", data)
	if err != nil {
		t.Error(err)
	} else {
		t.Logf("len(data): %d; received event: %+v", len(data), receivedAddrEv)
	}
}

func TestABI_MethodById(t *testing.T) {
	const abiJSON = `[
		{"type":"function","name":"receive","constant":false,"inputs":[{"name":"memo","type":"bytes"}],"outputs":[],"payable":true,"stateMutability":"payable"},
		{"type":"event","name":"received","anonymous":false,"inputs":[{"indexed":false,"name":"sender","type":"address"},{"indexed":false,"name":"amount","type":"uint256"},{"indexed":false,"name":"memo","type":"bytes"}]},
		{"type":"function","name":"fixedArrStr","constant":true,"inputs":[{"name":"str","type":"string"},{"name":"fixedArr","type":"uint256[2]"}]},
		{"type":"function","name":"fixedArrBytes","constant":true,"inputs":[{"name":"str","type":"bytes"},{"name":"fixedArr","type":"uint256[2]"}]},
		{"type":"function","name":"mixedArrStr","constant":true,"inputs":[{"name":"str","type":"string"},{"name":"fixedArr","type":"uint256[2]"},{"name":"dynArr","type":"uint256[]"}]},
		{"type":"function","name":"doubleFixedArrStr","constant":true,"inputs":[{"name":"str","type":"string"},{"name":"fixedArr1","type":"uint256[2]"},{"name":"fixedArr2","type":"uint256[3]"}]},
		{"type":"function","name":"multipleMixedArrStr","constant":true,"inputs":[{"name":"str","type":"string"},{"name":"fixedArr1","type":"uint256[2]"},{"name":"dynArr","type":"uint256[]"},{"name":"fixedArr2","type":"uint256[3]"}]},
		{"type":"function","name":"balance","constant":true},
		{"type":"function","name":"send","constant":false,"inputs":[{"name":"amount","type":"uint256"}]},
		{"type":"function","name":"test","constant":false,"inputs":[{"name":"number","type":"uint32"}]},
		{"type":"function","name":"string","constant":false,"inputs":[{"name":"inputs","type":"string"}]},
		{"type":"function","name":"bool","constant":false,"inputs":[{"name":"inputs","type":"bool"}]},
		{"type":"function","name":"address","constant":false,"inputs":[{"name":"inputs","type":"address"}]},
		{"type":"function","name":"uint64[2]","constant":false,"inputs":[{"name":"inputs","type":"uint64[2]"}]},
		{"type":"function","name":"uint64[]","constant":false,"inputs":[{"name":"inputs","type":"uint64[]"}]},
		{"type":"function","name":"foo","constant":false,"inputs":[{"name":"inputs","type":"uint32"}]},
		{"type":"function","name":"bar","constant":false,"inputs":[{"name":"inputs","type":"uint32"},{"name":"string","type":"uint16"}]},
		{"type":"function","name":"_slice","constant":false,"inputs":[{"name":"inputs","type":"uint32[2]"}]},
		{"type":"function","name":"__slice256","constant":false,"inputs":[{"name":"inputs","type":"uint256[2]"}]},
		{"type":"function","name":"sliceAddress","constant":false,"inputs":[{"name":"inputs","type":"address[]"}]},
		{"type":"function","name":"sliceMultiAddress","constant":false,"inputs":[{"name":"a","type":"address[]"},{"name":"b","type":"address[]"}]}
	]
`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}
	for name, m := range abi.Methods {
		a := fmt.Sprintf("%v", m)
		m2, err := abi.MethodById(m.Id())
		if err != nil {
			t.Fatalf("Failed to look up ABI method: %v", err)
		}
		b := fmt.Sprintf("%v", m2)
		if a != b {
			t.Errorf("Method %v (id %v) not 'findable' by id in ABI", name, common.ToHex(m.Id()))
		}
	}
}

var jsonEventTransfer = []byte(`{
  "anonymous": false,
  "inputs": [
    {
      "indexed": true, "name": "from", "type": "address"
    }, {
      "indexed": true, "name": "to", "type": "address"
    }, {
      "indexed": false, "name": "value", "type": "uint256"
  }],
  "name": "Transfer",
  "type": "event"
}`)

var jsonEventPledge = []byte(`{
  "anonymous": false,
  "inputs": [{
      "indexed": false, "name": "who", "type": "address"
    }, {
      "indexed": false, "name": "wad", "type": "uint128"
    }, {
      "indexed": false, "name": "currency", "type": "bytes3"
  }],
  "name": "Pledge",
  "type": "event"
}`)

var jsonEventMixedCase = []byte(`{
	"anonymous": false,
	"inputs": [{
		"indexed": false, "name": "value", "type": "uint256"
	  }, {
		"indexed": false, "name": "_value", "type": "uint256"
	  }, {
		"indexed": false, "name": "Value", "type": "uint256"
	}],
	"name": "MixedCase",
	"type": "event"
  }`)

// 1000000
var transferData1 = "00000000000000000000000000000000000000000000000000000000000f4240"

// "0x00Ce0d46d924CC8437c806721496599FC3FFA268", 2218516807680, "usd"
var pledgeData1 = "00000000000000000000000000ce0d46d924cc8437c806721496599fc3ffa2680000000000000000000000000000000000000000000000000000020489e800007573640000000000000000000000000000000000000000000000000000000000"

// 1000000,2218516807680,1000001
var mixedCaseData1 = "00000000000000000000000000000000000000000000000000000000000f42400000000000000000000000000000000000000000000000000000020489e8000000000000000000000000000000000000000000000000000000000000000f4241"

func TestEventId(t *testing.T) {
	var table = []struct {
		definition   string
		expectations map[string]common.Hash
	}{
		{
			definition: `[
			{ "type" : "event", "name" : "balance", "inputs": [{ "name" : "in", "type": "uint256" }] },
			{ "type" : "event", "name" : "check", "inputs": [{ "name" : "t", "type": "address" }, { "name": "b", "type": "uint256" }] }
			]`,
			expectations: map[string]common.Hash{
				"balance": crypto.Keccak256Hash([]byte("balance(uint256)")),
				"check":   crypto.Keccak256Hash([]byte("check(address,uint256)")),
			},
		},
	}

	for _, test := range table {
		abi, err := JSON(strings.NewReader(test.definition))
		if err != nil {
			t.Fatal(err)
		}

		for name, event := range abi.Events {
			if event.Id() != test.expectations[name] {
				t.Errorf("expected id to be %x, got %x", test.expectations[name], event.Id())
			}
		}
	}
}

// TestEventMultiValueWithArrayUnpack verifies that array fields will be counted after parsing array.
func TestEventMultiValueWithArrayUnpack(t *testing.T) {
	definition := `[{"name": "test", "type": "event", "inputs": [{"indexed": false, "name":"value1", "type":"uint8[2]"},{"indexed": false, "name":"value2", "type":"uint8"}]}]`
	type testStruct struct {
		Value1 [2]uint8
		Value2 uint8
	}
	abi, err := JSON(strings.NewReader(definition))
	require.NoError(t, err)
	var b bytes.Buffer
	var i uint8 = 1
	for ; i <= 3; i++ {
		b.Write(packNum(reflect.ValueOf(i)))
	}
	var rst testStruct
	require.NoError(t, abi.Unpack(&rst, "test", b.Bytes()))
	require.Equal(t, [2]uint8{1, 2}, rst.Value1)
	require.Equal(t, uint8(3), rst.Value2)
}

func TestEventTupleUnpack(t *testing.T) {

	type EventTransfer struct {
		Value *big.Int
	}

	type EventTransferWithTag struct {
		// this is valid because `value` is not exportable,
		// so value is only unmarshalled into `Value1`.
		value  *big.Int
		Value1 *big.Int `abi:"value"`
	}

	type BadEventTransferWithSameFieldAndTag struct {
		Value  *big.Int
		Value1 *big.Int `abi:"value"`
	}

	type BadEventTransferWithDuplicatedTag struct {
		Value1 *big.Int `abi:"value"`
		Value2 *big.Int `abi:"value"`
	}

	type BadEventTransferWithEmptyTag struct {
		Value *big.Int `abi:""`
	}

	type EventPledge struct {
		Who      common.Address
		Wad      *big.Int
		Currency [3]byte
	}

	type BadEventPledge struct {
		Who      string
		Wad      int
		Currency [3]byte
	}

	type EventMixedCase struct {
		Value1 *big.Int `abi:"value"`
		Value2 *big.Int `abi:"_value"`
		Value3 *big.Int `abi:"Value"`
	}

	bigint := new(big.Int)
	bigintExpected := big.NewInt(1000000)
	bigintExpected2 := big.NewInt(2218516807680)
	bigintExpected3 := big.NewInt(1000001)
	addr := common.HexToAddress("0x00Ce0d46d924CC8437c806721496599FC3FFA268")
	var testCases = []struct {
		data     string
		dest     interface{}
		expected interface{}
		jsonLog  []byte
		error    string
		name     string
	}{{
		transferData1,
		&EventTransfer{},
		&EventTransfer{Value: bigintExpected},
		jsonEventTransfer,
		"",
		"Can unpack ERC20 Transfer event into structure",
	}, {
		transferData1,
		&[]interface{}{&bigint},
		&[]interface{}{&bigintExpected},
		jsonEventTransfer,
		"",
		"Can unpack ERC20 Transfer event into slice",
	}, {
		transferData1,
		&EventTransferWithTag{},
		&EventTransferWithTag{Value1: bigintExpected},
		jsonEventTransfer,
		"",
		"Can unpack ERC20 Transfer event into structure with abi: tag",
	}, {
		transferData1,
		&BadEventTransferWithDuplicatedTag{},
		&BadEventTransferWithDuplicatedTag{},
		jsonEventTransfer,
		"struct: abi tag in 'Value2' already mapped",
		"Can not unpack ERC20 Transfer event with duplicated abi tag",
	}, {
		transferData1,
		&BadEventTransferWithSameFieldAndTag{},
		&BadEventTransferWithSameFieldAndTag{},
		jsonEventTransfer,
		"abi: multiple variables maps to the same abi field 'value'",
		"Can not unpack ERC20 Transfer event with a field and a tag mapping to the same abi variable",
	}, {
		transferData1,
		&BadEventTransferWithEmptyTag{},
		&BadEventTransferWithEmptyTag{},
		jsonEventTransfer,
		"struct: abi tag in 'Value' is empty",
		"Can not unpack ERC20 Transfer event with an empty tag",
	}, {
		pledgeData1,
		&EventPledge{},
		&EventPledge{
			addr,
			bigintExpected2,
			[3]byte{'u', 's', 'd'}},
		jsonEventPledge,
		"",
		"Can unpack Pledge event into structure",
	}, {
		pledgeData1,
		&[]interface{}{&common.Address{}, &bigint, &[3]byte{}},
		&[]interface{}{
			&addr,
			&bigintExpected2,
			&[3]byte{'u', 's', 'd'}},
		jsonEventPledge,
		"",
		"Can unpack Pledge event into slice",
	}, {
		pledgeData1,
		&[3]interface{}{&common.Address{}, &bigint, &[3]byte{}},
		&[3]interface{}{
			&addr,
			&bigintExpected2,
			&[3]byte{'u', 's', 'd'}},
		jsonEventPledge,
		"",
		"Can unpack Pledge event into an array",
	}, {
		pledgeData1,
		&[]interface{}{new(int), 0, 0},
		&[]interface{}{},
		jsonEventPledge,
		"abi: cannot unmarshal common.Address in to int",
		"Can not unpack Pledge event into slice with wrong types",
	}, {
		pledgeData1,
		&BadEventPledge{},
		&BadEventPledge{},
		jsonEventPledge,
		"abi: cannot unmarshal common.Address in to string",
		"Can not unpack Pledge event into struct with wrong filed types",
	}, {
		pledgeData1,
		&[]interface{}{common.Address{}, new(big.Int)},
		&[]interface{}{},
		jsonEventPledge,
		"abi: insufficient number of elements in the list/array for unpack, want 3, got 2",
		"Can not unpack Pledge event into too short slice",
	}, {
		pledgeData1,
		new(map[string]interface{}),
		&[]interface{}{},
		jsonEventPledge,
		"abi: cannot unmarshal tuple into map[string]interface {}",
		"Can not unpack Pledge event into map",
	}, {
		mixedCaseData1,
		&EventMixedCase{},
		&EventMixedCase{Value1: bigintExpected, Value2: bigintExpected2, Value3: bigintExpected3},
		jsonEventMixedCase,
		"",
		"Can unpack abi variables with mixed case",
	}}

	for _, tc := range testCases {
		assert := assert.New(t)
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := unpackTestEventData(tc.dest, tc.data, tc.jsonLog, assert)
			if tc.error == "" {
				assert.Nil(err, "Should be able to unpack event data.")
				assert.Equal(tc.expected, tc.dest, tc.name)
			} else {
				assert.EqualError(err, tc.error, tc.name)
			}
		})
	}
}

func unpackTestEventData(dest interface{}, hexData string, jsonEvent []byte, assert *assert.Assertions) error {
	data, err := hex.DecodeString(hexData)
	assert.NoError(err, "Hex data should be a correct hex-string")
	var e Event
	assert.NoError(json.Unmarshal(jsonEvent, &e), "Should be able to unmarshal event ABI")
	a := ABI{Events: map[string]Event{"e": e}}
	return a.Unpack(dest, "e", data)
}

type testResult struct {
	Values [2]*big.Int
	Value1 *big.Int
	Value2 *big.Int
}

type testCase struct {
	definition string
	want       testResult
}

func (tc testCase) encoded(intType, arrayType Type) []byte {
	var b bytes.Buffer
	if tc.want.Value1 != nil {
		val, _ := intType.pack(reflect.ValueOf(tc.want.Value1))
		b.Write(val)
	}

	if !reflect.DeepEqual(tc.want.Values, [2]*big.Int{nil, nil}) {
		val, _ := arrayType.pack(reflect.ValueOf(tc.want.Values))
		b.Write(val)
	}
	if tc.want.Value2 != nil {
		val, _ := intType.pack(reflect.ValueOf(tc.want.Value2))
		b.Write(val)
	}
	return b.Bytes()
}

// TestEventUnpackIndexed verifies that indexed field will be skipped by event decoder.
func TestEventUnpackIndexed(t *testing.T) {
	definition := `[{"name": "test", "type": "event", "inputs": [{"indexed": true, "name":"value1", "type":"uint8"},{"indexed": false, "name":"value2", "type":"uint8"}]}]`
	type testStruct struct {
		Value1 uint8
		Value2 uint8
	}
	abi, err := JSON(strings.NewReader(definition))
	require.NoError(t, err)
	var b bytes.Buffer
	b.Write(packNum(reflect.ValueOf(uint8(8))))
	var rst testStruct
	require.NoError(t, abi.Unpack(&rst, "test", b.Bytes()))
	require.Equal(t, uint8(0), rst.Value1)
	require.Equal(t, uint8(8), rst.Value2)
}

// TestEventIndexedWithArrayUnpack verifies that decoder will not overlow when static array is indexed input.
func TestEventIndexedWithArrayUnpack(t *testing.T) {
	definition := `[{"name": "test", "type": "event", "inputs": [{"indexed": true, "name":"value1", "type":"uint8[2]"},{"indexed": false, "name":"value2", "type":"string"}]}]`
	type testStruct struct {
		Value1 [2]uint8
		Value2 string
	}
	abi, err := JSON(strings.NewReader(definition))
	require.NoError(t, err)
	var b bytes.Buffer
	stringOut := "abc"
	// number of fields that will be encoded * 32
	b.Write(packNum(reflect.ValueOf(32)))
	b.Write(packNum(reflect.ValueOf(len(stringOut))))
	b.Write(common.RightPadBytes([]byte(stringOut), 32))

	var rst testStruct
	require.NoError(t, abi.Unpack(&rst, "test", b.Bytes()))
	require.Equal(t, [2]uint8{0, 0}, rst.Value1)
	require.Equal(t, stringOut, rst.Value2)
}

func TestABI_UnmarshalJSON(t *testing.T) {
	definition := `[{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"release","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
{"constant":false,"inputs":[{"name":"receiver","type":"string"},{"name":"destination","type":"string"}],"name":"deposit","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},
{"constant":true,"inputs":[{"name":"destination","type":"string"}],"name":"isValidType","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"view","type":"function"},
{"constant":false,"inputs":[{"name":"_type","type":"string"},{"name":"status","type":"bool"}],"name":"updateAvailableType","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
{"inputs":[{"name":"_owner","type":"address"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]`

	abi, err := JSON(strings.NewReader(definition))
	require.NoError(t, err)

	methodName := "release"

	for k, v := range abi.Methods {
		if k == methodName {
			for _, arg := range v.Inputs {
				println(fmt.Sprintf("Name: %v, Type: %v", arg.Name, arg.Type))
			}
		}
	}
}