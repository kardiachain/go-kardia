package abi

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/kardiachain/go-kardia/lib/common"
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
)

// Type is the reflection of the supported argument type
type Type struct {
	Elem *Type

	Kind reflect.Kind
	Type reflect.Type
	Size int
	T    byte // Our own type checking

	stringKind string // holds the unparsed string for deriving signatures
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

	if t.T == SliceTy || t.T == ArrayTy {
		var packed []byte

		for i := 0; i < v.Len(); i++ {
			val, err := t.Elem.pack(v.Index(i))
			if err != nil {
				return nil, err
			}
			packed = append(packed, val...)
		}
		if t.T == SliceTy {
			return packBytesSlice(packed, v.Len()), nil
		} else if t.T == ArrayTy {
			return packed, nil
		}
	}
	return packElement(t, v), nil
}

// NewType creates a new reflection type of abi type given in t.
func NewType(t string) (typ Type, err error) {
	// check that array brackets are equal if they exist
	if strings.Count(t, "[") != strings.Count(t, "]") {
		return Type{}, fmt.Errorf("invalid arg type in abi")
	}

	typ.stringKind = t

	// if there are brackets, get ready to go into slice/array mode and
	// recursively create the type
	if strings.Count(t, "[") != 0 {
		i := strings.LastIndex(t, "[")
		// recursively embed the type
		embeddedType, err := NewType(t[:i])
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
			typ.Kind = reflect.Slice
			typ.Elem = &embeddedType
			typ.Type = reflect.SliceOf(embeddedType.Type)
		} else if len(intz) == 1 {
			// is a array
			typ.T = ArrayTy
			typ.Kind = reflect.Array
			typ.Elem = &embeddedType
			typ.Size, err = strconv.Atoi(intz[0])
			if err != nil {
				return Type{}, fmt.Errorf("abi: error parsing variable size: %v", err)
			}
			typ.Type = reflect.ArrayOf(typ.Size, embeddedType.Type)
		} else {
			return Type{}, fmt.Errorf("invalid formatting of array type")
		}
		return typ, err
	}
	// parse the type and size of the abi-type.
	parsedType := typeRegex.FindAllStringSubmatch(t, -1)[0]
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
		typ.Kind, typ.Type = reflectIntKindAndType(false, varSize)
		typ.Size = varSize
		typ.T = IntTy
	case "uint":
		typ.Kind, typ.Type = reflectIntKindAndType(true, varSize)
		typ.Size = varSize
		typ.T = UintTy
	case "bool":
		typ.Kind = reflect.Bool
		typ.T = BoolTy
		typ.Type = reflect.TypeOf(bool(false))
	case "address":
		typ.Kind = reflect.Array
		typ.Type = addressT
		typ.Size = 20
		typ.T = AddressTy
	case "string":
		typ.Kind = reflect.String
		typ.Type = reflect.TypeOf("")
		typ.T = StringTy
	case "bytes":
		if varSize == 0 {
			typ.T = BytesTy
			typ.Kind = reflect.Slice
			typ.Type = reflect.SliceOf(reflect.TypeOf(byte(0)))
		} else {
			typ.T = FixedBytesTy
			typ.Kind = reflect.Array
			typ.Size = varSize
			typ.Type = reflect.ArrayOf(varSize, reflect.TypeOf(byte(0)))
		}
	case "function":
		typ.Kind = reflect.Array
		typ.T = FunctionTy
		typ.Size = 24
		typ.Type = reflect.ArrayOf(24, reflect.TypeOf(byte(0)))
	default:
		return Type{}, fmt.Errorf("unsupported arg type: %s", t)
	}

	return
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
	return common.PaddedBigBytes(math.U256(n), 32)
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
		return typeErr(formatSliceString(t.Kind, t.Size), val.Type())
	}

	if t.T == ArrayTy && val.Len() != t.Size {
		return typeErr(formatSliceString(t.Elem.Kind, t.Size), formatSliceString(val.Type().Elem().Kind(), val.Len()))
	}

	if t.Elem.T == SliceTy {
		if val.Len() > 0 {
			return sliceTypeCheck(*t.Elem, val.Index(0))
		}
	} else if t.Elem.T == ArrayTy {
		return sliceTypeCheck(*t.Elem, val.Index(0))
	}

	if elemKind := val.Type().Elem().Kind(); elemKind != t.Elem.Kind {
		return typeErr(formatSliceString(t.Elem.Kind, t.Size), val.Type())
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
	if t.Kind != value.Kind() {
		return typeErr(t.Kind, value.Kind())
	} else if t.T == FixedBytesTy && t.Size != value.Len() {
		return typeErr(t.Type, value.Type())
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

// requireAssignable assures that `dest` is a pointer and it's not an interface.
func requireAssignable(dst, src reflect.Value) error {
	if dst.Kind() != reflect.Ptr && dst.Kind() != reflect.Interface {
		return fmt.Errorf("abi: cannot unmarshal %v into %v", src.Type(), dst.Type())
	}
	return nil
}

// mapAbiToStringField maps abi to struct fields.
// first round: for each Exportable field that contains a `abi:""` tag
//   and this field name exists in the arguments, pair them together.
// second round: for each argument field that has not been already linked,
//   find what variable is expected to be mapped into, if it exists and has not been
//   used, pair them.
func mapAbiToStructFields(args Arguments, value reflect.Value) (map[string]string, error) {

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
		var ok bool
		var tagName string
		if tagName, ok = typ.Field(i).Tag.Lookup("abi"); !ok {
			continue
		}

		// check if tag is empty.
		if tagName == "" {
			return nil, fmt.Errorf("struct: abi tag in '%s' is empty", structFieldName)
		}

		// check which argument field matches with the abi tag.
		found := false
		for _, abiField := range args.NonIndexed() {
			if abiField.Name == tagName {
				if abi2struct[abiField.Name] != "" {
					return nil, fmt.Errorf("struct: abi tag in '%s' already mapped", structFieldName)
				}
				// pair them
				abi2struct[abiField.Name] = structFieldName
				struct2abi[structFieldName] = abiField.Name
				found = true
			}
		}

		// check if this tag has been mapped.
		if !found {
			return nil, fmt.Errorf("struct: abi tag '%s' defined but not found in abi", tagName)
		}

	}

	// second round ~~~
	for _, arg := range args {

		abiFieldName := arg.Name
		structFieldName := capitalise(abiFieldName)

		if structFieldName == "" {
			return nil, fmt.Errorf("abi: purely underscored output cannot unpack to struct")
		}

		// this abi has already been paired, skip it... unless there exists another, yet unassigned
		// struct field with the same field name. If so, raise an error:
		//    abi: [ { "name": "value" } ]
		//    struct { Value  *big.Int , Value1 *big.Int `abi:"value"`}
		if abi2struct[abiFieldName] != "" {
			if abi2struct[abiFieldName] != structFieldName &&
				struct2abi[structFieldName] == "" &&
				value.FieldByName(structFieldName).IsValid() {
				return nil, fmt.Errorf("abi: multiple variables maps to the same abi field '%s'", abiFieldName)
			}
			continue
		}

		// return an error if this struct field has already been paired.
		if struct2abi[structFieldName] != "" {
			return nil, fmt.Errorf("abi: multiple outputs mapping to the same struct field '%s'", structFieldName)
		}

		if value.FieldByName(structFieldName).IsValid() {
			// pair them
			abi2struct[abiFieldName] = structFieldName
			struct2abi[structFieldName] = abiFieldName
		} else {
			// not paired, but annotate as used, to detect cases like
			//   abi : [ { "name": "value" }, { "name": "_value" } ]
			//   struct { Value *big.Int }
			struct2abi[structFieldName] = abiFieldName
		}

	}

	return abi2struct, nil
}

// set attempts to assign src to dst by either setting, copying or otherwise.
//
// set is a bit more lenient when it comes to assignment and doesn't force an as
// strict ruleset as bare `reflect` does.
func set(dst, src reflect.Value, output Argument) error {
	dstType := dst.Type()
	srcType := src.Type()
	switch {
	case dstType.AssignableTo(srcType):
		dst.Set(src)
	case dstType.Kind() == reflect.Interface:
		dst.Set(src)
	case dstType.Kind() == reflect.Ptr:
		return set(dst.Elem(), src, output)
	default:
		return fmt.Errorf("abi: cannot unmarshal %v in to %v", src.Type(), dst.Type())
	}
	return nil
}

// requireUnpackKind verifies preconditions for unpacking `args` into `kind`
func requireUnpackKind(v reflect.Value, t reflect.Type, k reflect.Kind,
	args Arguments) error {

	switch k {
	case reflect.Struct:
	case reflect.Slice, reflect.Array:
		if minLen := args.LengthNonIndexed(); v.Len() < minLen {
			return fmt.Errorf("abi: insufficient number of elements in the list/array for unpack, want %d, got %d",
				minLen, v.Len())
		}
	default:
		return fmt.Errorf("abi: cannot unmarshal tuple into %v", t)
	}
	return nil
}
