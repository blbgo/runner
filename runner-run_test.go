package runner

import (
	"errors"
	"testing"

	"github.com/blbgo/testing/assert"
)

//********************
func TestNewAndEmptyRunError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.NotNil(b)
	errs := b.Run()
	a.Equal(1, len(errs))
	a.True(errors.Is(errs[0], ErrNoMain))
}

//********************
type testInterface2 interface{ Method() string }

func new1Consume2(i testInterface2) testInterface1 { return testStruct1{} }

func TestMissingDependencyError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.NoError(b.Add(new1Consume2))
	errs := b.Run()
	a.Equal(1, len(errs))
	a.True(errors.Is(errs[0], ErrNoProducerMakes), "Expecting", ErrNoProducerMakes, "got", errs[0])
}

//********************
type testStruct2 struct{}

func (r testStruct2) Method() string { return "testStruct2.Method" }

func new2Consume1(i testInterface1) testInterface2 { return testStruct2{} }

func TestCircularReferenceError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.NoError(b.Add(new1Consume2))
	a.NoError(b.Add(new2Consume1))
	errs := b.Run()
	a.Equal(2, len(errs))
	a.True(errors.Is(errs[0], ErrMissingDependency), "Expecting", ErrMissingDependency, "got", errs[0])
	a.True(errors.Is(errs[1], ErrMissingDependency), "Expecting", ErrMissingDependency, "got", errs[1])
}

//********************
func newNilProvider() testInterface1 { return nil }

func TestProviderReturnsNilError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.NoError(b.Add(newNilProvider))
	errs := b.Run()
	a.Equal(1, len(errs))
	a.True(errors.Is(errs[0], ErrProducerReturnedNil), "Expecting", ErrProducerReturnedNil, "got", errs[0])
}

//********************
type testMainError struct{}

var errMainError = errors.New("error from Main.Run")

func (r testMainError) Run() error { return errMainError }

func newMainError() Main { return testMainError{} }

func TestMainRunErrorReturned(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.NoError(b.Add(newMainError))
	errs := b.Run()
	a.Equal(1, len(errs))
	a.True(errors.Is(errs[0], errMainError), "Expecting", errMainError, "got", errs[0])
}

//********************
func new2() testInterface2 { return testStruct2{} }

func TestSliceWhenSingleNeededError(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.NoError(b.Add(new1Consume2))
	a.NoError(b.Add(new2))
	a.NoError(b.Add(new2))
	errs := b.Run()
	a.Equal(1, len(errs))
	a.True(errors.Is(errs[0], ErrNoProducerMakes), "Expecting", ErrNoProducerMakes, "got", errs[0])
}

//********************
func new1ConsumeSice2(i []testInterface2) testInterface1 { return testStruct1{} }

//****
type testStruct2Closer struct{}

func (r testStruct2Closer) Method() string { return "testStruct2Closer.Method" }

var errCloser = errors.New("error from Closer.Close")

func (r testStruct2Closer) Close() error { return errCloser }

func new2Closer() testInterface2 { return testStruct2Closer{} }

//****
type testStruct2DelayCloser struct{}

func (r testStruct2DelayCloser) Method() string { return "testStruct2DelayCloser.Method" }

func (r testStruct2DelayCloser) Close(doneChan chan<- error) { go sendErrDelayCloser(doneChan) }

var errDelayCloser = errors.New("error from DelayCloser.Close")

func sendErrDelayCloser(doneChan chan<- error) { doneChan <- errDelayCloser }

func new2DelayCloser() testInterface2 { return testStruct2DelayCloser{} }

//****
type testMain struct{}

func (r testMain) Run() error { return nil }

func newMain(i testInterface1) Main { return testMain{} }

func TestMainRun(t *testing.T) {
	a := assert.New(t)

	b := New()
	a.NoError(b.Add(new1ConsumeSice2))
	a.NoError(b.Add(new2))
	a.NoError(b.Add(new2Closer))
	a.NoError(b.Add(new2DelayCloser))
	a.NoError(b.Add(newMain))
	errs := b.Run()
	a.Equal(2, len(errs))
	a.True(errors.Is(errs[0], errDelayCloser), "Expecting", errDelayCloser, "got", errs[0])
	a.True(errors.Is(errs[1], errCloser), "Expecting", errCloser, "got", errs[1])
}
