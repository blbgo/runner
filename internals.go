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

type runner struct {
	closeTimeout  time.Duration
	produceCounts map[reflect.Type]int
	provideSlice  map[reflect.Type]bool
	producers     []reflect.Value
	values        map[reflect.Type]reflect.Value
	closers       []interface{}
}

// defaultCloseTimeout is the default timeout duration to wait for general.DelayCloser complete
// notifications
const defaultCloseTimeout = 20 * time.Second

var nilValue = reflect.ValueOf(nil)
var xvalueType = reflect.TypeOf((*reflect.Value)(nil)).Elem()
var errorType = reflect.TypeOf((*error)(nil)).Elem()
var mainType = reflect.TypeOf((*Main)(nil)).Elem()

// new creates a Runner
func new() *runner {
	return &runner{
		closeTimeout:  defaultCloseTimeout,
		produceCounts: make(map[reflect.Type]int),
		provideSlice:  make(map[reflect.Type]bool),
		values:        make(map[reflect.Type]reflect.Value),
	}
}

// Add see Runner interface doc
func (r *runner) add(producer interface{}) error {
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
func (r *runner) run() []error {
	errs := r.build()
	if errs != nil {
		return r.close(errs)
	}

	// get the Main interface
	mainValue, ok := r.values[mainType]
	if !ok {
		errs = append(errs, ErrNoMain)
		return r.close(errs)
	}
	main, ok := mainValue.Interface().(Main)
	if !ok {
		errs = append(
			errs,
			errors.New("BUG Main interface found but can not type assert to Main"),
		)
		return r.close(errs)
	}

	// values no longer needed, set to null to maybe free memory
	r.values = nil

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
	// nil out producers, produceCounts, and provideSlice so memory can be garbage collected
	r.producers = nil
	r.produceCounts = nil
	r.provideSlice = nil
	return nil
}

// resolveProvider finds inputs, calls, and processes the results for a single provider
func (r *runner) resolveProvider(provider reflect.Value) error {
	providerType := provider.Type()
	in := make([]reflect.Value, providerType.NumIn())
	for i := 0; i < len(in); i++ {
		param, err := r.findParam(providerType.In(i))
		if err != nil {
			return err
		}
		in[i] = param
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
		err := r.handleProvidedValue(result)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) findParam(paramType reflect.Type) (reflect.Value, error) {
	kind := paramType.Kind()
	if kind == reflect.Slice {
		if r.produceCounts[paramType.Elem()] > 0 {
			return nilValue, fmt.Errorf("%w type: %v", ErrMissingDependency, paramType)
		}
	} else if r.produceCounts[paramType] > 0 {
		return nilValue, fmt.Errorf("%w type: %v", ErrMissingDependency, paramType)
	}

	param, ok := r.values[paramType]
	if !ok {
		if kind != reflect.Slice {
			// bad will be no way to resolve this type ever
			return nilValue, fmt.Errorf("%w type: %v", ErrNoProducerMakes, paramType)
		}
		// need a slice of something that will not be produced, seems like providing an empty slice
		// would be the correct behavior instead of an error
		return reflect.MakeSlice(paramType, 0, 0), nil
	}
	return param, nil
}

func (r *runner) handleProvidedValue(value reflect.Value) error {
	providedValueType := value.Type()
	waitForCount := r.produceCounts[providedValueType]
	if waitForCount <= 0 {
		return fmt.Errorf("BUG not waiting for produced type: %v", providedValueType)
	}
	// nothing wants a slice but a slice is what there will be
	if waitForCount > 1 && !r.provideSlice[providedValueType] {
		r.provideSlice[providedValueType] = true
	}
	r.produceCounts[providedValueType] = waitForCount - 1
	r.saveIfCloser(value)
	if !r.provideSlice[providedValueType] {
		r.values[providedValueType] = value
		return nil
	}
	providedSliceType := reflect.SliceOf(providedValueType)
	aValue, ok := r.values[providedSliceType]
	if ok {
		r.values[providedSliceType] = reflect.Append(aValue, value)
		return nil
	}
	r.values[providedSliceType] = reflect.Append(
		reflect.MakeSlice(providedSliceType, 0, waitForCount),
		value,
	)
	return nil
}

func (r *runner) saveIfCloser(value reflect.Value) {
	valueInterface := value.Interface()
	switch valueInterface.(type) {
	case io.Closer:
		r.closers = append(r.closers, valueInterface)
	case general.DelayCloser:
		r.closers = append(r.closers, valueInterface)
	}
}

// Close closes any values in the runner that implement the io.Closer or general.DelayCloser
// interfaces.  They are closed in reverse creation order.  This will insure a values close will
// be called before any of its dependencies.
func (r *runner) close(errs []error) []error {
	doneChan := make(chan error)
	timer := time.NewTimer(r.closeTimeout)
	for i := len(r.closers) - 1; i >= 0; i-- {
		switch v := r.closers[i].(type) {
		case io.Closer:
			err := v.Close()
			if err != nil {
				errs = append(errs, err)
			}
		case general.DelayCloser:
			v.Close(doneChan)
			select {
			case err, ok := <-doneChan:
				if !ok {
					return append(errs, errors.New("BUG runner DelayCloser doneChan closed"))
				}
				if err != nil {
					errs = append(errs, err)
				}
			case <-timer.C:
				return append(errs, ErrDelayCloserTimeout)
			}
		default:
			errs = append(errs, errors.New("BUG runner has non closer in closers"))
		}
	}
	return errs
}
