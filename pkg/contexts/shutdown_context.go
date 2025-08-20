package contexts

import (
	"context"
	"os"
	"sync"
	"syscall"

	"github.com/cardinalby/depo/internal/signals"
)

// ErrSignalReceived is an error that is used as a cancellation cause for NewShutdownContext when the
// application receives a syscall.SIGINT or syscall.SIGTERM signal
type ErrSignalReceived struct {
	os.Signal
}

func (e ErrSignalReceived) Error() string {
	return e.Signal.String() + " signal received"
}

// NewShutdownContext creates a new context that gets cancelled when the application receives
// syscall.SIGINT or syscall.SIGTERM signal. context.Cause() will return ErrSignalReceived in this case
func NewShutdownContext(parent context.Context) (context.Context, context.CancelCauseFunc) {
	signalsChan := make(chan os.Signal, 2)
	signals.Notify(signalsChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancelInternal := context.WithCancelCause(parent)
	var once sync.Once
	cancel := func(cause error) {
		once.Do(func() {
			signals.Stop(signalsChan)
			cancelInternal(cause)
			close(signalsChan)
		})
	}
	go func() {
		select {
		case sig, ok := <-signalsChan:
			if ok {
				cancel(ErrSignalReceived{Signal: sig})
			} else {
				cancel(context.Cause(parent))
			}

		case <-parent.Done():
			cancel(context.Cause(parent))
		}
	}()
	return ctx, cancel
}
