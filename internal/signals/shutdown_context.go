package signals

import (
	"context"
	"os"
	"sync"
	"syscall"
)

type ErrSignalReceived struct {
	os.Signal
}

func (e ErrSignalReceived) Error() string {
	return e.Signal.String() + " signal received"
}

func NewShutdownContext() (context.Context, context.CancelCauseFunc) {
	signalsChan := make(chan os.Signal, 2)
	Notify(signalsChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancelInternal := context.WithCancelCause(context.Background())
	var once sync.Once
	cancel := func(cause error) {
		once.Do(func() {
			Stop(signalsChan)
			cancelInternal(cause)
			close(signalsChan)
		})
	}
	go func() {
		if sig, ok := <-signalsChan; ok {
			cancel(ErrSignalReceived{Signal: sig})
		}
	}()
	return ctx, cancel
}
