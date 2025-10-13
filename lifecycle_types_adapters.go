package depo

import (
	"context"
	"reflect"
	"sync/atomic"
)

type trustedAsyncStarter interface {
	trustedAsyncStarterDeclaration()
}

type trustedAsyncCloser interface {
	trustedAsyncCloserDeclaration()
}

type lcAdapterStringer interface {
	// is used to describe the implementation that was passed to the lifecycle hook in error messages.
	// If name is self-explanatory, method is empty string. Of name contains the name of a user type,
	// method contains the method name of the type that is used
	getLcAdapterImplNameAndMethod() (name string, method string)
}

// wrap fn into structs to make struct pointers hashable
type fnReadinessRunnable struct {
	fn func(ctx context.Context, onReady func()) (err error)
}

func (f *fnReadinessRunnable) Run(ctx context.Context, onReady func()) (err error) {
	return f.fn(ctx, onReady)
}

func (f *fnReadinessRunnable) getLcAdapterImplNameAndMethod() (name string, method string) {
	return "ReadinessRunFn", ""
}

type fnRunnable struct {
	fn func(ctx context.Context) (err error)
}

func (f *fnRunnable) Run(ctx context.Context) (err error) {
	return f.fn(ctx)
}

func (f *fnRunnable) getLcAdapterImplNameAndMethod() (name string, method string) {
	return "RunFn", ""
}

type fnStarter struct {
	fn func(ctx context.Context) (err error)
}

func (f *fnStarter) Start(ctx context.Context) (err error) {
	return f.fn(ctx)
}

func (f *fnStarter) getLcAdapterImplNameAndMethod() (name string, method string) {
	return "StartFn", ""
}

type fnCloser struct {
	fn func()
}

func (f *fnCloser) Close() {
	f.fn()
}

func (f *fnCloser) getLcAdapterImplNameAndMethod() (name string, method string) {
	return "CloseFn", ""
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

func (pr *phasedReadinessRunnable) getLcAdapterImplNameAndMethod() (name string, method string) {
	if adapterStringer, ok := pr.runnable.(lcAdapterStringer); ok {
		return adapterStringer.getLcAdapterImplNameAndMethod()
	}
	return reflect.TypeOf(pr.runnable).String(), lcRunMethodName
}

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

func (pr *phasedRunnable) getLcAdapterImplNameAndMethod() (name string, method string) {
	if adapterStringer, ok := pr.runnable.(lcAdapterStringer); ok {
		return adapterStringer.getLcAdapterImplNameAndMethod()
	}
	return reflect.TypeOf(pr.runnable).String(), lcRunMethodName
}
