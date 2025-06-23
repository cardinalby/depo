package depo

import (
	"context"
	"sync/atomic"
)

type trustedAsyncStarter interface {
	trustedAsyncStarterDeclaration()
}

type trustedAsyncCloser interface {
	trustedAsyncCloserDeclaration()
}

// wrap fn into structs to make struct pointers hashable
type fnReadinessRunnable struct {
	fn func(ctx context.Context, onReady func()) (err error)
}

func (f *fnReadinessRunnable) Run(ctx context.Context, onReady func()) (err error) {
	return f.fn(ctx, onReady)
}

type fnRunnable struct {
	fn func(ctx context.Context) (err error)
}

func (f *fnRunnable) Run(ctx context.Context) (err error) {
	return f.fn(ctx)
}

type fnStarter struct {
	fn func(ctx context.Context) (err error)
}

func (f *fnStarter) Start(ctx context.Context) (err error) {
	return f.fn(ctx)
}

type fnCloser struct {
	fn func()
}

func (f *fnCloser) Close() {
	f.fn()
}

type fnWaiter struct {
	fn func() (err error)
}

func (f *fnWaiter) wait() (err error) {
	return f.fn()
}

type phasedReadinessRunnable struct {
	runnable     ReadinessRunnable
	runCtx       context.Context
	cancelRunCtx context.CancelCauseFunc
	runResult    chan error
	isRunning    atomic.Bool
}

func newPhasedReadinessRunnable(runnable ReadinessRunnable) *phasedReadinessRunnable {
	return &phasedReadinessRunnable{
		runnable: runnable,
	}
}

func (pr *phasedReadinessRunnable) Start(ctx context.Context) error {
	if pr.isRunning.Swap(true) {
		return errAlreadyRunning
	}
	pr.runCtx, pr.cancelRunCtx = context.WithCancelCause(context.Background())
	pr.runResult = make(chan error, 1)
	readySignal := make(chan struct{})

	go func() {
		pr.runResult <- pr.runnable.Run(pr.runCtx, func() {
			close(readySignal)
		})
		pr.isRunning.Store(false)
	}()
	select {
	case err := <-pr.runResult:
		return err
	case <-readySignal:
		return nil
	case <-ctx.Done():
		pr.cancelRunCtx(context.Cause(ctx))
		return <-pr.runResult
	}
}

func (pr *phasedReadinessRunnable) wait() error {
	runResult := pr.runResult
	return <-runResult
}

func (pr *phasedReadinessRunnable) close(cause error) {
	pr.cancelRunCtx(cause)
}

func (pr *phasedReadinessRunnable) trustedAsyncCloserDeclaration() {}

type phasedRunnable struct {
	runnable     Runnable
	runCtx       context.Context
	cancelRunCtx context.CancelCauseFunc
	runResult    chan error
	isRunning    atomic.Bool
}

func newPhasedRunnable(runnable Runnable) *phasedRunnable {
	return &phasedRunnable{
		runnable: runnable,
	}
}

func (pr *phasedRunnable) Start(_ context.Context) error {
	if pr.isRunning.Swap(true) {
		return errAlreadyRunning
	}
	pr.runCtx, pr.cancelRunCtx = context.WithCancelCause(context.Background())
	pr.runResult = make(chan error, 1)

	go func() {
		pr.runResult <- pr.runnable.Run(pr.runCtx)
		pr.isRunning.Store(false)
	}()
	return nil
}

func (pr *phasedRunnable) trustedAsyncStarterDeclaration() {}

func (pr *phasedRunnable) wait() error {
	return <-pr.runResult
}

func (pr *phasedRunnable) close(cause error) {
	pr.cancelRunCtx(cause)
}

func (pr *phasedRunnable) trustedAsyncCloserDeclaration() {}

func (pr *phasedRunnable) trustedNoOpCloserAfterDoneDeclaration() {}
