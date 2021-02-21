// Package runner is a dependency management tool for go
package runner

import (
	"errors"
)

// Main is an interface that must be provided by one (and only one) producer passed to Run.
type Main interface {
	Run() error
}

// ErrProducerNil indicates nil was passed to Add
var ErrProducerNil = errors.New("producer nil")

// ErrProducerNotFunc indicates a non function was passed to Add
var ErrProducerNotFunc = errors.New("producer not function")

// ErrProducerInvalidReturns indicates a function returning invalid values was passed to Add
var ErrProducerInvalidReturns = errors.New("producer may only return interfaces and an optional error")

// ErrProducerInvalidInputs indicates a function with invalid inputs was passed to Add
var ErrProducerInvalidInputs = errors.New("producer inputs must be interface or slice of interfaces")

// ErrMissingDependency indicates there is a missing dependency, it will be wrapped so the missing
// type can be included
var ErrMissingDependency = errors.New("missing dependency")

// ErrNoProducerMakes indicates no producer makes a required dependency, it will be wrapped so the
// missing type can be included
var ErrNoProducerMakes = errors.New("no producer makes")

// ErrProducerReturnedNil indicates a producer returned nil instead of a valid interface
var ErrProducerReturnedNil = errors.New("producer returned nil value")

// ErrNoMain indicates no Main was provided
var ErrNoMain = errors.New("No Main interface provided")

// ErrDelayCloserTimeout indicates a timeout waiting for general.DelayCloser(s) to complete
var ErrDelayCloserTimeout = errors.New("timeout before all DelayCloser results")

// Run runs a dependency stack
//
// producers must all be functions. These functions may only have interface or slice of interfaces
// as there parameters and may return any number of interfaces and an optional error as the last
// return value.
//
// Run first calls all producer functions exactly once.  If any producer functions return an error
// that error will be returned. If the parameters of a producer function can not be produced by
// other producer function Run will return with appropriate error(s). This may be caused by
// circular references.
//
// If all producers are successfully called and a Main interface is among the produced values its
// Run method will be called exactly once. If no Main interface was produced an error will be
// returned.
//
// Finally all produced values that implement io.Closer or general.DelayCloser will have the Close
// method of those interfaces called. This will be done in the opposite order that the values were
// produced insuring that a values Close will be called before any of its dependencies.
//
// The error slice returned may have errors from the producer functions or an error from the
// Main.Run function.  In either case there my also be errors from the Close functions of produced
// values.
func Run(producers []interface{}) []error {
	runner := new()

	for _, v := range producers {
		err := runner.add(v)
		if err != nil {
			return []error{err}
		}
	}

	return runner.run()
}
