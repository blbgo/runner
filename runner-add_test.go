package runner

import (
	"testing"

	"github.com/blbergwall/assert"
)

//********************
func TestAddNilError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.Equal(b.Add(nil), ErrProducerNil)
}

//********************
func TestAddNonFuncError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.Equal(b.Add(2), ErrProducerNotFunc)
}

//********************
func nonInterfaceOut() int { return 5 }

func TestAddNonInterfaceReturnedError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.Equal(b.Add(nonInterfaceOut), ErrProducerInvalidReturns)
}

//********************
type testInterface1 interface{ Method() string }
type testStruct1 struct{}

func (r testStruct1) Method() string { return "testStruct1.Method" }

func nonInterfaceIn(int) testInterface1 { return testStruct1{} }
func TestAddNonInterfaceInError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.Equal(b.Add(nonInterfaceIn), ErrProducerInvalidInputs)
}
