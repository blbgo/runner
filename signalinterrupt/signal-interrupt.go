package signalinterrupt

import (
	"errors"
	"os"
	"os/signal"

	"github.com/blbgo/general"
)

type signalInterrupt struct {
	general.Shutdowner
	signalChan chan os.Signal
	doneChan   chan<- error
}

// NewSignalInterrupt creates a signalInterrupt and returns it as a general.DelayCloser. This
// allows ctrl-C to cleanly shutdown a command line program.
func NewSignalInterrupt(shutdowner general.Shutdowner) general.DelayCloser {
	r := &signalInterrupt{
		Shutdowner: shutdowner,
		signalChan: make(chan os.Signal, 1),
	}

	signal.Notify(r.signalChan, os.Interrupt)

	go r.run()

	return r
}

func (r *signalInterrupt) Close(doneChan chan<- error) {
	r.doneChan = doneChan

	signal.Stop(r.signalChan)
	close(r.signalChan)
}

func (r *signalInterrupt) run() {
	// wait for signal or chanel close
	_, ok := <-r.signalChan

	// got signal?
	if ok {
		r.Shutdown(errors.New("Interrupt signal received"))
	}

	// wait for chanel to close
	for ok {
		_, ok = <-r.signalChan
	}

	r.doneChan <- nil
}
