# runner

A dependency injection tool for go.

[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](http://godoc.org/github.com/blbergwall/runner)
[![Go Report Card](https://goreportcard.com/badge/github.com/blbergwall/depend)](https://goreportcard.com/report/github.com/blbergwall/runner)
[![License](http://img.shields.io/badge/license-mit-blue.svg)](https://github.com/blbergwall/runner/blob/master/LICENSE.txt)

## Overview

Create a Runner, *Add* dependency and subsystem producing functions, call *Run* run the system.

## Install

```shell
go get github.com/blbergwall/runner
```

## Usage

Import the runner package

```go
import "github.com/blbergwall/runner"
```

Create a new Runner

```go
runner := runner.New()
```

Next producer functions must be added

Given the following interface, implementation, and producer function

```go
type testInterface interface { Method() string }

type testStruct struct { field string }
func (r *testStruct) Method() string { return r.field }

func newTestInterface() testInterface { return &testStruct{ field: "testInterface" } }
```

Add it to the runner with

```go
runner.Add(newTestInterface)
```

Also need a runner.Main implementation and producer

```go
type mainImplementor struct{testInterface}
func (r *mainImplementor) Run() error { fmt.Println(r.Method()) }
}

func newMain(ti testInterface) runner.Main { return &mainImplementor{ testInterface: ti } }
```

Add it to the runner with

```go
runner.Add(newMain)
```

Now that all producer functions have been added the runner can be run

```go
errs := runner.Run()
```

This causes all added functions to be called once.  If any functions return an error or any
functions have dependencies that are not produced by other functions error(s) will be returned. If
all functions are successfully called and a runner.Main interface was produced its Run method is
called (if no runner.Main was produced an error is returned).

Finally (either because there was an error at any point or because the runner.Main Run method
returned) any produced values that implement io.Closer or general.DelayCloser will have there
Close method called.

## License

[MIT](https://github.com/blbergwall/depend/blob/master/LICENSE.txt)
