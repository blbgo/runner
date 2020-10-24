package shutdownermain

import (
	"sync"

	"github.com/blbgo/general"
	"github.com/blbgo/runner"
)

type shutdowner struct {
	sync.Mutex
	done         bool
	shutdownChan chan error
}

type main <-chan error

// NewShutdownerMain provides general.Shutdowner and runner.Main
func NewShutdownerMain() (general.Shutdowner, runner.Main) {
	r := &shutdowner{
		shutdownChan: make(chan error),
	}
	return r, main(r.shutdownChan)
}

// **************** implement general.Shutdowner on shutdowner

// Shutdown tells the runner stack to shutdown (the Main.Run method will return). An error can
// be provided that will be returned by Main.Run (first call to Shutdown only).  nil can be
// provided to cause Main.Run to return nil.
func (r *shutdowner) Shutdown(err error) {
	r.Lock()
	defer r.Unlock()
	if !r.done {
		r.done = true
		r.shutdownChan <- err
		close(r.shutdownChan)
	}
}

// **************** implement runner.Main on main

// Run waits for somthing (an error or nil) to come through the channel and then returns it
func (r main) Run() error {
	// wait for first sent item and return it
	return <-r
}
