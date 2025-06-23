package signals

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownContext_Signal(t *testing.T) {
	tests := []struct {
		name   string
		signal os.Signal
	}{
		{
			name:   "SIGINT",
			signal: syscall.SIGINT,
		},
		{
			name:   "SIGTERM",
			signal: syscall.SIGTERM,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Do not run signal tests in parallel
			// t.Parallel()
			ctx, cancel := NewShutdownContext()
			require.Equal(t, 1, MockSubscribersCount())
			defer cancel(nil)

			go func() {
				// A small delay to allow the signal handler to be set up.
				time.Sleep(100 * time.Millisecond)
				SendMockSignal(tt.signal)
			}()

			select {
			case <-ctx.Done():
				err := context.Cause(ctx)
				require.Error(t, err)
				var signalErr ErrSignalReceived
				require.ErrorAs(t, err, &signalErr)
				require.Equal(t, tt.signal, signalErr.Signal)
				require.Equal(t, tt.signal.String()+" signal received", err.Error())
				require.Equal(t, 0, MockSubscribersCount())
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for signal")
			}
		})
	}
}

func TestShutdownContext_Cancel(t *testing.T) {
	ctx, cancel := NewShutdownContext()
	require.Equal(t, 1, MockSubscribersCount())
	testCause := errors.New("test_cause")

	cancel(testCause)

	select {
	case <-ctx.Done():
		assert.ErrorIs(t, context.Cause(ctx), testCause)
		require.Equal(t, 0, MockSubscribersCount())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for context cancellation")
	}
}
