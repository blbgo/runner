// Package runner is a dependency management tool for go
package runner

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/blbgo/general"
)

// Runner allows adding all dependencies and then running the stack
type Runner interface {
	// SetCloseTimeout sets the deration to wait for all general.DelayCloser results to be
	// reported.  If not set the default of 20 seconds will be used.
	SetCloseTimeout(closeTimeout time.Duration)

	// Add takes producer functions that can return interfaces so those interfaces can be provided
	// to other producer functions.  Producer functions may only have interface or slice of
	// interfaces as there parameters and may return any number of interfaces and an optional
	// error as there last return value.
	Add(producer interface{}) error

	// Run first calls all added producer functions exactly once.  If any producer functions
	// return an error that error will be returned.  If the inputs of a producer function can not
	// be produced by other producer function Run will return with appropriate error(s).  This may
	// be caused by circular references.
	//
	// If all producer functions are successfully called and a Main interface is among the
	// produced values its Run method will be called exactly once.  If no Main interface was
	// produced an error will be returned.
	//
	// Finally all produced values that implement io.Closer or general.DelayCloser will have the
	// Close method of those interfaces called.
	//
	// The error slice returned may have errors from the producer functions or an error from the
	// Main.Run function.  In either case there my also be errors from the Close functions.
	Run() []error
}

// Main is an interface that must be provided by one of the items added with Runner.Add for the
// Runner.Run method to work.
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

type runner struct {
	closeTimeout  time.Duration
	produceCounts map[reflect.Type]int
	provideSlice  map[reflect.Type]bool
	producers     []reflect.Value
	values        map[reflect.Type]reflect.Value
}

// defaultCloseTimeout is the default timeout duration to wait for general.DelayCloser complete
// notifications
const defaultCloseTimeout = 20 * time.Second

var nilValue = reflect.ValueOf(nil)
var valueType = reflect.TypeOf((*reflect.Value)(nil)).Elem()
var errorType = reflect.TypeOf((*error)(nil)).Elem()
var mainType = reflect.TypeOf((*Main)(nil)).Elem()

// New creates a Runner
func New() Runner {
	return &runner{
		closeTimeout:  defaultCloseTimeout,
		produceCounts: make(map[reflect.Type]int),
		provideSlice:  make(map[reflect.Type]bool),
		values:        make(map[reflect.Type]reflect.Value),
	}
}

func (r *runner) SetCloseTimeout(closeTimeout time.Duration) {
	r.closeTimeout = closeTimeout
}

// Add see Runner interface doc
func (r *runner) Add(producer interface{}) error {
	itemType := reflect.TypeOf(producer)
	if itemType == nil {
		return ErrProducerNil
	}
	if itemType.Kind() != reflect.Func {
		return ErrProducerNotFunc
	}

	// validate and note return types
	outCount := itemType.NumOut()
	// last return type error, ignore for now
	if outCount > 0 && itemType.Out(outCount-1) == errorType {
		outCount--
	}
	for i := 0; i < outCount; i++ {
		outType := itemType.Out(i)
		if outType.Kind() != reflect.Interface {
			return ErrProducerInvalidReturns
		}
		r.produceCounts[outType]++
	}

	// validate inputs and note slice requirements
	for inCount := itemType.NumIn() - 1; inCount >= 0; inCount-- {
		inType := itemType.In(inCount)
		inKind := inType.Kind()
		switch {
		case inKind == reflect.Slice && inType.Elem().Kind() == reflect.Interface:
			r.provideSlice[inType.Elem()] = true
		case inKind == reflect.Interface:
			// nothing to do just valid
		default:
			return ErrProducerInvalidInputs
		}
	}

	r.producers = append(r.producers, reflect.ValueOf(producer))
	return nil
}

// Run see Runner interface doc
func (r *runner) Run() []error {
	errs := r.build()
	if errs != nil {
		return r.close(errs)
	}

	mainValue, ok := r.values[mainType]
	if !ok {
		errs = append(errs, ErrNoMain)
		return r.close(errs)
	}
	main, ok := mainValue.Interface().(Main)
	if !ok {
		errs = append(errs, errors.New("BUG Main interface found but can not type assert to Main"))
		return r.close(errs)
	}
	err := main.Run()
	if err != nil {
		errs = append(errs, err)
	}
	return r.close(errs)
}

// build calls all added functions once.  If any functions return errors or
// any functions have dependencies that have not been added or there are any
// circular references a slice of errors will be returned.
func (r *runner) build() []error {
	var waitingProducers []reflect.Value
	var errs []error
	for len(r.producers) > 0 {
		for _, value := range r.producers {
			err := r.resolveProvider(value)
			if errors.Is(err, ErrMissingDependency) {
				errs = append(errs, err)
				waitingProducers = append(waitingProducers, value)
			} else if err != nil {
				return []error{err}
			}
		}

		// did not resolve any producers
		if len(waitingProducers) == len(r.producers) {
			return errs
		}
		errs = errs[:0]
		waitingProducers, r.producers = r.producers[:0], waitingProducers
	}
	// nil out producers so memory can be garbage collected
	r.producers = nil
	return nil
}

// resolveProvider finds inputs, calls, and processes the results for a single provider
func (r *runner) resolveProvider(provider reflect.Value) error {
	providerType := provider.Type()
	in := make([]reflect.Value, providerType.NumIn())
	for i := 0; i < len(in); i++ {
		paramType := providerType.In(i)
		if paramType.Kind() == reflect.Slice {
			if r.produceCounts[paramType.Elem()] > 0 {
				return fmt.Errorf("%w type: %v", ErrMissingDependency, paramType)
			}
		} else if r.produceCounts[paramType] > 0 {
			return fmt.Errorf("%w type: %v", ErrMissingDependency, paramType)
		}

		anIn, ok := r.values[paramType]
		if !ok {
			// bad will be no way to resolve this type ever
			return fmt.Errorf("%w type: %v", ErrNoProducerMakes, paramType)
		}
		in[i] = anIn
	}
	results := provider.Call(in)
	resultsCount := len(results)
	if resultsCount > 0 && providerType.Out(resultsCount-1) == errorType {
		result := results[resultsCount-1]
		if !result.IsNil() {
			return result.Interface().(error)
		}
		resultsCount--
	}
	for i := 0; i < resultsCount; i++ {
		result := results[i]
		if result.IsNil() {
			return fmt.Errorf("%w type: %v", ErrProducerReturnedNil, providerType.Out(i))
		}
		resultType := result.Type()
		waitForCount := r.produceCounts[resultType]
		if waitForCount <= 0 {
			return fmt.Errorf("BUG not waiting for produced type: %v", resultType)
		}
		// nothing wants a slice but a slice is what there will be
		if waitForCount > 1 && !r.provideSlice[resultType] {
			r.provideSlice[resultType] = true
		}
		r.produceCounts[resultType] = waitForCount - 1
		if r.provideSlice[resultType] {
			resultType = reflect.SliceOf(resultType)
			aValue, ok := r.values[resultType]
			if ok {
				r.values[resultType] = reflect.Append(aValue, result)
			} else {
				r.values[resultType] = reflect.Append(
					reflect.MakeSlice(resultType, 0, waitForCount),
					result,
				)
			}
		} else {
			r.values[resultType] = result
		}

	}
	return nil
}

// Close closes any values in the runner that implement the io.Closer or general.DelayCloser
// interfaces
func (r *runner) close(errs []error) []error {
	doneChan := make(chan error)
	delayCount := 0
	for i, v := range r.values {
		switch i.Kind() {
		case reflect.Interface:
			delay, err := closeValue(v.Interface(), doneChan)
			if err != nil {
				errs = append(errs, err)
			} else if delay {
				delayCount++
			}
		case reflect.Slice:
			for j := 0; j < v.Len(); j++ {
				delay, err := closeValue(v.Index(j).Interface(), doneChan)
				if err != nil {
					errs = append(errs, err)
				} else if delay {
					delayCount++
				}
			}
		}
	}
	timer := time.NewTimer(r.closeTimeout)
	for ; delayCount > 0; delayCount-- {
		select {
		case err, ok := <-doneChan:
			if !ok {
				errs = append(errs, errors.New("DelayCloser doneChan closed"))
				break
			}
			if err != nil {
				errs = append(errs, err)
			}
		case <-timer.C:
			errs = append(errs, ErrDelayCloserTimeout)
			break
		}
	}
	return errs
}

func closeValue(value interface{}, doneChan chan error) (bool, error) {
	closer, ok := value.(io.Closer)
	if ok {
		return false, closer.Close()
	}
	delayCloser, ok := value.(general.DelayCloser)
	if ok {
		delayCloser.Close(doneChan)
		return true, nil
	}
	return false, nil
}
