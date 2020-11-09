# runner

A dependency management tool for go

[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](http://godoc.org/github.com/blbgo/runner)
[![Go Report Card](https://goreportcard.com/badge/github.com/blbgo/runner)](https://goreportcard.com/report/github.com/blbgo/runner)
[![License](http://img.shields.io/badge/license-mit-blue.svg)](https://github.com/blbgo/runner/blob/master/LICENSE.txt)

## Overview

Run a system built from a list of functions that provide all the dependencies.

## Install

```shell
go get github.com/blbgo/runner
```

## Usage

Import the runner package

```go
import "github.com/blbgo/runner"
```

Given the following interface, implementation, and producer function

```go
type testInterface interface { Method() string }

type testStruct struct { field string }
func (r *testStruct) Method() string { return r.field }

func newTestInterface() testInterface { return &testStruct{ field: "testInterface" } }
```

And a runner.Main implementation that uses it

```go
type mainImplementor struct{testInterface}
func (r *mainImplementor) Run() error { fmt.Println(r.Method()) }

func newMain(ti testInterface) runner.Main { return &mainImplementor{ testInterface: ti } }
```

This system can be run with

```go
errs := runner.Run([]interface{}{newTestInterface, newMain})
for _, err := range errs {
	fmt.Println(err)
}
```

This causes each function passed to Run to be called once.  If any functions return an error or any
functions have dependencies that are not produced by other functions error(s) will be returned. If
all functions are successfully called and a runner.Main interface was produced its Run method is
called (if no runner.Main was produced an error is returned).

Finally (either because there was an error at any point or because the runner.Main Run method
returned) any produced values that implement io.Closer or general.DelayCloser will have there
Close method called.

The functions passed to main must only take parameters of interface type or slice of interfaces.
They must return only interfaces and an optional error as the last return value.

Note that if the same interface is provided more than once then a slice of that interface is what
must be depended on.

## License

[MIT](https://github.com/blbgo/runner/blob/master/LICENSE.txt)
