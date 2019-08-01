package common

import "testing"

func TestSet_Has(t *testing.T) {
	type structA struct {
		name string
	}

	s := NewSet(1)
	a := structA{name: "Hello"}
	var b interface{} = a
	s.Add(b)
	if !s.Has(a) {
		t.Error("expect a is found")
	}
}
