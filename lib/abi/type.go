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
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/kardiachain/go-kardiamain/lib/common"
)

var (
	errBadBool = errors.New("abi: improperly encoded boolean value")
)

// Type enumerator
const (
	IntTy byte = iota
	UintTy
	BoolTy
	StringTy
	SliceTy
	ArrayTy
	AddressTy
	FixedBytesTy
	BytesTy
	HashTy
	FixedPointTy
	FunctionTy
	TupleTy
)

// Type is the reflection of the supported argument type
type Type struct {
	Elem *Type

	Size int
	T    byte // Our own type checking

	stringKind string // holds the unparsed string for deriving signatures

	// Tuple relative fields
	TupleRawName  string       // Raw struct name defined in source code, may be empty.
	TupleElems    []*Type      // Type information of all tuple fields
	TupleRawNames []string     // Raw field name of all tuple fields
	TupleType     reflect.Type // Underlying struct of the tuple
}

var (
	// typeRegex parses the abi sub types
	typeRegex = regexp.MustCompile("([a-zA-Z]+)(([0-9]+)(x([0-9]+))?)?")
)

// String implements Stringer
func (t Type) String() (out string) {
	return t.stringKind
}

func (t Type) pack(v reflect.Value) ([]byte, error) {
	// dereference pointer first if it's a pointer
	v = indirect(v)

	if err := typeCheck(t, v); err != nil {
		return nil, err
	}

	switch t.T {
	case SliceTy, ArrayTy:
		var ret []byte

		if t.requiresLengthPrefix() {
			// append length
			ret = append(ret, packNum(reflect.ValueOf(v.Len()))...)
		}

		// calculate offset if any
		offset := 0
		offsetReq := isDynamicType(*t.Elem)
		if offsetReq {
			offset = getTypeSize(*t.Elem) * v.Len()
		}
		var tail []byte
		for i := 0; i < v.Len(); i++ {
			val, err := t.Elem.pack(v.Index(i))
			if err != nil {
				return nil, err
			}
			if !offsetReq {
				ret = append(ret, val...)
				continue
			}
			ret = append(ret, packNum(reflect.ValueOf(offset))...)
			offset += len(val)
			tail = append(tail, val...)
		}
		return append(ret, tail...), nil
	case TupleTy:
		// (T1,...,Tk) for k >= 0 and any types T1, …, Tk
		// enc(X) = head(X(1)) ... head(X(k)) tail(X(1)) ... tail(X(k))
		// where X = (X(1), ..., X(k)) and head and tail are defined for Ti being a static
		// type as
		//     head(X(i)) = enc(X(i)) and tail(X(i)) = "" (the empty string)
		// and as
		//     head(X(i)) = enc(len(head(X(1)) ... head(X(k)) tail(X(1)) ... tail(X(i-1))))
		//     tail(X(i)) = enc(X(i))
		// otherwise, i.e. if Ti is a dynamic type.
		fieldmap, err := mapArgNamesToStructFields(t.TupleRawNames, v)
		if err != nil {
			return nil, err
		}
		// Calculate prefix occupied size.
		offset := 0
		for _, elem := range t.TupleElems {
			offset += getTypeSize(*elem)
		}
		var ret, tail []byte
		for i, elem := range t.TupleElems {
			field := v.FieldByName(fieldmap[t.TupleRawNames[i]])
			if !field.IsValid() {
				return nil, fmt.Errorf("field %s for tuple not found in the given struct", t.TupleRawNames[i])
			}
			val, err := elem.pack(field)
			if err != nil {
				return nil, err
			}
			if isDynamicType(*elem) {
				ret = append(ret, packNum(reflect.ValueOf(offset))...)
				tail = append(tail, val...)
				offset += len(val)
			} else {
				ret = append(ret, val...)
			}
		}
		return append(ret, tail...), nil

	default:
		return packElement(t, v), nil
	}
}

// NewType creates a new reflection type of abi type given in t.
func NewType(t string, internalType string, components []ArgumentMarshaling) (typ Type, err error) {
	// check that array brackets are equal if they exist
	if strings.Count(t, "[") != strings.Count(t, "]") {
		return Type{}, fmt.Errorf("invalid arg type in abi")
	}

	typ.stringKind = t

	// if there are brackets, get ready to go into slice/array mode and
	// recursively create the type
	if strings.Count(t, "[") != 0 {
		// Note internalType can be empty here.
		subInternal := internalType
		if i := strings.LastIndex(internalType, "["); i != -1 {
			subInternal = subInternal[:i]
		}

		i := strings.LastIndex(t, "[")
		// recursively embed the type
		embeddedType, err := NewType(t[:i], subInternal, components)
		if err != nil {
			return Type{}, err
		}
		// grab the last cell and create a type from there
		sliced := t[i:]
		// grab the slice size with regexp
		re := regexp.MustCompile("[0-9]+")
		intz := re.FindAllString(sliced, -1)

		if len(intz) == 0 {
			// is a slice
			typ.T = SliceTy
			typ.Elem = &embeddedType
			typ.stringKind = embeddedType.stringKind + sliced
		} else if len(intz) == 1 {
			// is a array
			typ.T = ArrayTy
			typ.Elem = &embeddedType
			typ.Size, err = strconv.Atoi(intz[0])
			if err != nil {
				return Type{}, fmt.Errorf("abi: error parsing variable size: %v", err)
			}
			typ.stringKind = embeddedType.stringKind + sliced
		} else {
			return Type{}, fmt.Errorf("invalid formatting of array type")
		}
		return typ, err
	}
	// parse the type and size of the abi-type.

	matches := typeRegex.FindAllStringSubmatch(t, -1)
	if len(matches) == 0 {
		return Type{}, fmt.Errorf("invalid type '%v'", t)
	}
	parsedType := matches[0]

	// varSize is the size of the variable
	var varSize int
	if len(parsedType[3]) > 0 {
		var err error
		varSize, err = strconv.Atoi(parsedType[2])
		if err != nil {
			return Type{}, fmt.Errorf("abi: error parsing variable size: %v", err)
		}
	} else {
		if parsedType[0] == "uint" || parsedType[0] == "int" {
			// this should fail because it means that there's something wrong with
			// the abi type (the compiler should always format it to the size...always)
			return Type{}, fmt.Errorf("unsupported arg type: %s", t)
		}
	}
	// varType is the parsed abi type
	switch varType := parsedType[1]; varType {
	case "int":
		typ.Size = varSize
		typ.T = IntTy
	case "uint":
		typ.Size = varSize
		typ.T = UintTy
	case "bool":
		typ.T = BoolTy
	case "address":
		typ.Size = 20
		typ.T = AddressTy
	case "string":
		typ.T = StringTy
	case "bytes":
		if varSize == 0 {
			typ.T = BytesTy
		} else {
			typ.T = FixedBytesTy
			typ.Size = varSize
		}
	case "function":
		typ.T = FunctionTy
		typ.Size = 24
	case "tuple":
		var (
			fields     []reflect.StructField
			elems      []*Type
			names      []string
			expression string // canonical parameter expression
		)
		expression += "("
		overloadedNames := make(map[string]string)
		for idx, c := range components {
			cType, err := NewType(c.Type, c.InternalType, c.Components)
			if err != nil {
				return Type{}, err
			}
			fieldName, err := overloadedArgName(c.Name, overloadedNames)
			if err != nil {
				return Type{}, err
			}
			overloadedNames[fieldName] = fieldName
			fields = append(fields, reflect.StructField{
				Name: fieldName, // reflect.StructOf will panic for any exported field.
				Type: cType.GetType(),
				Tag:  reflect.StructTag("json:\"" + c.Name + "\""),
			})
			elems = append(elems, &cType)
			names = append(names, c.Name)
			expression += cType.stringKind
			if idx != len(components)-1 {
				expression += ","
			}
		}
		expression += ")"
		typ.TupleType = reflect.StructOf(fields)
		typ.TupleElems = elems
		typ.TupleRawNames = names
		typ.T = TupleTy
		typ.stringKind = expression

		const structPrefix = "struct "
		// After solidity 0.5.10, a new field of abi "internalType"
		// is introduced. From that we can obtain the struct name
		// user defined in the source code.
		if internalType != "" && strings.HasPrefix(internalType, structPrefix) {
			// Foo.Bar type definition is not allowed in golang,
			// convert the format to FooBar
			typ.TupleRawName = strings.Replace(internalType[len(structPrefix):], ".", "", -1)
		}

	default:
		return Type{}, fmt.Errorf("unsupported arg type: %s", t)
	}

	return
}

// GetType returns the reflection type of the ABI type.
func (t Type) GetType() reflect.Type {
	switch t.T {
	case IntTy:
		_, inType := reflectIntKindAndType(false, t.Size)
		return inType
	case UintTy:
		_, uInType := reflectIntKindAndType(true, t.Size)
		return uInType
	case BoolTy:
		return reflect.TypeOf(false)
	case StringTy:
		return reflect.TypeOf("")
	case SliceTy:
		return reflect.SliceOf(t.Elem.GetType())
	case ArrayTy:
		return reflect.ArrayOf(t.Size, t.Elem.GetType())
	case TupleTy:
		return t.TupleType
	case AddressTy:
		return reflect.TypeOf(common.Address{})
	case FixedBytesTy:
		return reflect.ArrayOf(t.Size, reflect.TypeOf(byte(0)))
	case BytesTy:
		return reflect.SliceOf(reflect.TypeOf(byte(0)))
	case HashTy:
		// hashtype currently not used
		return reflect.ArrayOf(32, reflect.TypeOf(byte(0)))
	case FixedPointTy:
		// fixedpoint type currently not used
		return reflect.ArrayOf(32, reflect.TypeOf(byte(0)))
	case FunctionTy:
		return reflect.ArrayOf(24, reflect.TypeOf(byte(0)))
	default:
		panic("Invalid type")
	}
}

func overloadedArgName(rawName string, names map[string]string) (string, error) {
	fieldName := ToCamelCase(rawName)
	if fieldName == "" {
		return "", errors.New("abi: purely anonymous or underscored field is not supported")
	}
	// Handle overloaded fieldNames
	_, ok := names[fieldName]
	for idx := 0; ok; idx++ {
		fieldName = fmt.Sprintf("%s%d", ToCamelCase(rawName), idx)
		_, ok = names[fieldName]
	}
	return fieldName, nil
}

// requireLengthPrefix returns whether the type requires any sort of length
// prefixing.
func (t Type) requiresLengthPrefix() bool {
	return t.T == StringTy || t.T == BytesTy || t.T == SliceTy
}

//==============================================================================
// Helpers
//==============================================================================

var (
	bigT      = reflect.TypeOf(&big.Int{})
	derefbigT = reflect.TypeOf(big.Int{})
	uint8T    = reflect.TypeOf(uint8(0))
	uint16T   = reflect.TypeOf(uint16(0))
	uint32T   = reflect.TypeOf(uint32(0))
	uint64T   = reflect.TypeOf(uint64(0))
	int8T     = reflect.TypeOf(int8(0))
	int16T    = reflect.TypeOf(int16(0))
	int32T    = reflect.TypeOf(int32(0))
	int64T    = reflect.TypeOf(int64(0))
	addressT  = reflect.TypeOf(common.Address{})
)

// U256 converts a big Int into a 256bit KVM number.
func U256(n *big.Int) []byte {
	return common.PaddedBigBytes(common.U256(n), 32)
}

// formatSliceString formats the reflection kind with the given slice size
// and returns a formatted string representation.
func formatSliceString(kind reflect.Kind, sliceSize int) string {
	if sliceSize == -1 {
		return fmt.Sprintf("[]%v", kind)
	}
	return fmt.Sprintf("[%d]%v", sliceSize, kind)
}

// sliceTypeCheck checks that the given slice can by assigned to the reflection
// type in t.
func sliceTypeCheck(t Type, val reflect.Value) error {
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return typeErr(formatSliceString(t.GetType().Kind(), t.Size), val.Type())
	}

	if t.T == ArrayTy && val.Len() != t.Size {
		return typeErr(formatSliceString(t.Elem.GetType().Kind(), t.Size), formatSliceString(val.Type().Elem().Kind(), val.Len()))
	}

	if t.Elem.T == SliceTy || t.Elem.T == ArrayTy {
		if val.Len() > 0 {
			return sliceTypeCheck(*t.Elem, val.Index(0))
		}
	}

	if elemKind := val.Type().Elem().Kind(); elemKind != t.Elem.GetType().Kind() {
		return typeErr(formatSliceString(t.Elem.GetType().Kind(), t.Size), val.Type())
	}
	return nil
}

// typeCheck checks that the given reflection value can be assigned to the reflection
// type in t.
func typeCheck(t Type, value reflect.Value) error {
	if t.T == SliceTy || t.T == ArrayTy {
		return sliceTypeCheck(t, value)
	}

	// Check base type validity. Element types will be checked later on.
	if t.GetType().Kind() != value.Kind() {
		return typeErr(t.GetType().Kind(), value.Kind())
	} else if t.T == FixedBytesTy && t.Size != value.Len() {
		return typeErr(t.GetType(), value.Type())
	} else {
		return nil
	}
}

// typeErr returns a formatted type casting error.
func typeErr(expected, got interface{}) error {
	return fmt.Errorf("abi: cannot use %v as type %v as argument", got, expected)
}

// mustArrayToBytesSlice creates a new byte slice with the exact same size as value
// and copies the bytes in value to the new slice.
func mustArrayToByteSlice(value reflect.Value) reflect.Value {
	slice := reflect.MakeSlice(reflect.TypeOf([]byte{}), value.Len(), value.Len())
	reflect.Copy(slice, value)
	return slice
}

// indirect recursively dereferences the value until it either gets the value
// or finds a big.Int
func indirect(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Ptr && v.Elem().Type() != derefbigT {
		return indirect(v.Elem())
	}
	return v
}

// reflectIntKind returns the reflect using the given size and
// unsignedness.
func reflectIntKindAndType(unsigned bool, size int) (reflect.Kind, reflect.Type) {
	switch size {
	case 8:
		if unsigned {
			return reflect.Uint8, uint8T
		}
		return reflect.Int8, int8T
	case 16:
		if unsigned {
			return reflect.Uint16, uint16T
		}
		return reflect.Int16, int16T
	case 32:
		if unsigned {
			return reflect.Uint32, uint32T
		}
		return reflect.Int32, int32T
	case 64:
		if unsigned {
			return reflect.Uint64, uint64T
		}
		return reflect.Int64, int64T
	}
	return reflect.Ptr, bigT
}

// set attempts to assign src to dst by either setting, copying or otherwise.
//
// set is a bit more lenient when it comes to assignment and doesn't force an as
// strict ruleset as bare `reflect` does.
func set(dst, src reflect.Value) error {
	dstType := dst.Type()
	srcType := src.Type()
	switch {
	case dstType.Kind() == reflect.Interface && dst.Elem().IsValid():
		return set(dst.Elem(), src)
	case dstType.Kind() == reflect.Ptr && dstType.Elem() != reflect.TypeOf(big.Int{}):
		return set(dst.Elem(), src)
	case srcType.AssignableTo(dstType) && dst.CanSet():
		dst.Set(src)
	case dstType.Kind() == reflect.Slice && srcType.Kind() == reflect.Slice && dst.CanSet():
		return setSlice(dst, src)
	case dstType.Kind() == reflect.Array:
		return setArray(dst, src)
	case dstType.Kind() == reflect.Struct:
		return setStruct(dst, src)
	default:
		return fmt.Errorf("abi: cannot unmarshal %v in to %v", src.Type(), dst.Type())
	}
	return nil
}

// setSlice attempts to assign src to dst when slices are not assignable by default
// e.g. src: [][]byte -> dst: [][15]byte
// setSlice ignores if we cannot copy all of src' elements.
func setSlice(dst, src reflect.Value) error {
	slice := reflect.MakeSlice(dst.Type(), src.Len(), src.Len())
	for i := 0; i < src.Len(); i++ {
		if src.Index(i).Kind() == reflect.Struct {
			if err := set(slice.Index(i), src.Index(i)); err != nil {
				return err
			}
		} else {
			// e.g. [][32]uint8 to []common.Hash
			if err := set(slice.Index(i), src.Index(i)); err != nil {
				return err
			}
		}
	}
	if dst.CanSet() {
		dst.Set(slice)
		return nil
	}
	return errors.New("Cannot set slice, destination not settable")
}

func setArray(dst, src reflect.Value) error {
	array := reflect.New(dst.Type()).Elem()
	min := src.Len()
	if src.Len() > dst.Len() {
		min = dst.Len()
	}
	for i := 0; i < min; i++ {
		if err := set(array.Index(i), src.Index(i)); err != nil {
			return err
		}
	}
	if dst.CanSet() {
		dst.Set(array)
		return nil
	}
	return errors.New("Cannot set array, destination not settable")
}

func setStruct(dst, src reflect.Value) error {
	for i := 0; i < src.NumField(); i++ {
		srcField := src.Field(i)
		dstField := dst.Field(i)
		if !dstField.IsValid() || !srcField.IsValid() {
			return fmt.Errorf("Could not find src field: %v value: %v in destination", srcField.Type().Name(), srcField)
		}
		if err := set(dstField, srcField); err != nil {
			return err
		}
	}
	return nil
}

// isDynamicType returns true if the type is dynamic.
// The following types are called “dynamic”:
// * bytes
// * string
// * T[] for any T
// * T[k] for any dynamic T and any k >= 0
// * (T1,...,Tk) if Ti is dynamic for some 1 <= i <= k
func isDynamicType(t Type) bool {
	if t.T == TupleTy {
		for _, elem := range t.TupleElems {
			if isDynamicType(*elem) {
				return true
			}
		}
		return false
	}
	return t.T == StringTy || t.T == BytesTy || t.T == SliceTy || (t.T == ArrayTy && isDynamicType(*t.Elem))
}

// getTypeSize returns the size that this type needs to occupy.
// We distinguish static and dynamic types. Static types are encoded in-place
// and dynamic types are encoded at a separately allocated location after the
// current block.
// So for a static variable, the size returned represents the size that the
// variable actually occupies.
// For a dynamic variable, the returned size is fixed 32 bytes, which is used
// to store the location reference for actual value storage.
func getTypeSize(t Type) int {
	if t.T == ArrayTy && !isDynamicType(*t.Elem) {
		// Recursively calculate type size if it is a nested array
		if t.Elem.T == ArrayTy {
			return t.Size * getTypeSize(*t.Elem)
		}
		return t.Size * 32
	} else if t.T == TupleTy && !isDynamicType(t) {
		total := 0
		for _, elem := range t.TupleElems {
			total += getTypeSize(*elem)
		}
		return total
	}
	return 32
}

// mapArgNamesToStructFields maps a slice of argument names to struct fields.
// first round: for each Exportable field that contains a `abi:""` tag
//   and this field name exists in the given argument name list, pair them together.
// second round: for each argument name that has not been already linked,
//   find what variable is expected to be mapped into, if it exists and has not been
//   used, pair them.
// Note this function assumes the given value is a struct value.
func mapArgNamesToStructFields(argNames []string, value reflect.Value) (map[string]string, error) {
	typ := value.Type()

	abi2struct := make(map[string]string)
	struct2abi := make(map[string]string)

	// first round ~~~
	for i := 0; i < typ.NumField(); i++ {
		structFieldName := typ.Field(i).Name

		// skip private struct fields.
		if structFieldName[:1] != strings.ToUpper(structFieldName[:1]) {
			continue
		}
		// skip fields that have no abi:"" tag.
		tagName, ok := typ.Field(i).Tag.Lookup("abi")
		if !ok {
			continue
		}
		// check if tag is empty.
		if tagName == "" {
			return nil, fmt.Errorf("struct: abi tag in '%s' is empty", structFieldName)
		}
		// check which argument field matches with the abi tag.
		found := false
		for _, arg := range argNames {
			if arg == tagName {
				if abi2struct[arg] != "" {
					return nil, fmt.Errorf("struct: abi tag in '%s' already mapped", structFieldName)
				}
				// pair them
				abi2struct[arg] = structFieldName
				struct2abi[structFieldName] = arg
				found = true
			}
		}
		// check if this tag has been mapped.
		if !found {
			return nil, fmt.Errorf("struct: abi tag '%s' defined but not found in abi", tagName)
		}
	}

	// second round ~~~
	for _, argName := range argNames {

		structFieldName := ToCamelCase(argName)

		if structFieldName == "" {
			return nil, fmt.Errorf("abi: purely underscored output cannot unpack to struct")
		}

		// this abi has already been paired, skip it... unless there exists another, yet unassigned
		// struct field with the same field name. If so, raise an error:
		//    abi: [ { "name": "value" } ]
		//    struct { Value  *big.Int , Value1 *big.Int `abi:"value"`}
		if abi2struct[argName] != "" {
			if abi2struct[argName] != structFieldName &&
				struct2abi[structFieldName] == "" &&
				value.FieldByName(structFieldName).IsValid() {
				return nil, fmt.Errorf("abi: multiple variables maps to the same abi field '%s'", argName)
			}
			continue
		}

		// return an error if this struct field has already been paired.
		if struct2abi[structFieldName] != "" {
			return nil, fmt.Errorf("abi: multiple outputs mapping to the same struct field '%s'", structFieldName)
		}

		if value.FieldByName(structFieldName).IsValid() {
			// pair them
			abi2struct[argName] = structFieldName
			struct2abi[structFieldName] = argName
		} else {
			// not paired, but annotate as used, to detect cases like
			//   abi : [ { "name": "value" }, { "name": "_value" } ]
			//   struct { Value *big.Int }
			struct2abi[structFieldName] = argName
		}
	}
	return abi2struct, nil
}
