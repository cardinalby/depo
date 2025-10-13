package depo

import (
	"context"
)

// Starter is a component that should be successfully started before dependent components can Start.
// Examples are:
// - DB migrations
// - one-time initialization tasks (check DB schema, kafka topics configs, etc...),
// - configuration providers
// - connections to DBs/queues that need to be established (normally have corresponding Closer)
// UseLifecycle().AddStarter() can be used to register a Starter in the component's lifecycle.
// Starter can be combined with Closer (Close will be called only if Start was successful).
type Starter interface {
	// Start should:
	// - block until the component is ready to be used and its dependents can Start
	// - return a non-nil error if the component can't be started (ctx.Err() if ctx is done)
	Start(ctx context.Context) error
}

// Closer is a component that should be stopped gracefully before stopping its dependencies during shutdown.
// Examples are:
// - buffering message queue producers/caches/repos that should flush their buffers
// - connection pools with lazy connections (sql.DB)
// - externally started timers, opened files, etc...
// UseLifecycle().AddCloser() can be used to register a Closer in the component's lifecycle.
// Closer can be combined with Starter (Close will be called only if Start was successful).
type Closer interface {
	// Close should block until the component is stopped.
	// If it's a Starter, Close will be called only if Start was successful.
	Close()
}

// Runnable is an alternative and more powerful sync semantics for "immediately ready Starter" + Closer with
// health tracking in a single Run method. Once Run is called, the component unblocks dependents` starts.
// UseLifecycle().AddRunnable() can be used to register a Runnable in the component's lifecycle.
type Runnable interface {
	// Run should:
	// - block until the component completes its job or is stopped via `ctx` cancellation.
	// - return non-nil error if the component fails and needs to trigger shutdown of other components
	// - shut down gracefully and return `ctx.Err()` if `ctx` is canceled. context.Cause(ctx) can be used to
	//   determine the cause of the shutdown.
	// - return nil error if the component finishes successfully and doesn't want to trigger shutdown of other components.
	//   This default can be overridden by OptNilRunResultAsError option
	Run(ctx context.Context) error
}

// ReadinessRunnable is an alternative and more powerful sync semantics for Starter + Closer with
// health tracking in a single Run method.
// It notifies its readiness (when it doesn't block starting of dependents anymore) via `onReady` callback.
// UseLifecycle().AddReadinessRunnable() can be used to register a ReadinessRunnable in the component's lifecycle.
type ReadinessRunnable interface {
	// Run should:
	// - block until the component completes its job or is stopped via `ctx` cancellation.
	// - call `onReady` callback once the component is ready and its dependents can Start
	// - return non-nil error if the component fails and needs to trigger shutdown
	// - shut down gracefully and return `ctx.Err()` if `ctx` is canceled. context.Cause(ctx) can be used to
	//   determine the cause of the shutdown.
	// - return nil error if the component finishes successfully and doesn't want to trigger shutdown of other components.
	//   This default can be overridden by OptNilRunResultAsError option
	Run(ctx context.Context, onReady func()) error
}

const lcStartMethodName = "Start"
const lcRunMethodName = "Run"
