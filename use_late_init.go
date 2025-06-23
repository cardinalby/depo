package depo

import (
	"fmt"

	"github.com/cardinalby/depo/internal/runtm"
)

func newErrNilLateInit() error {
	return fmt.Errorf("%w lateInitFn", errNilValue)
}

// UseLateInit can be called inside `provide` callback to register a `lateInitFn`
// that will be called after `provider` function returns but before `getComponent` returns.
// It's used to resolve cyclic dependencies between components:
//  1. first you provide a component without initializing it with dependencies
//  2. the component is available for other components to be used as a dependency
//  3. then `lateInitFn` is called to finish component's initialization requesting dependencies that have
//     been already initialized at this point
//
// UseLateInit resolves only construction time cyclic dependencies,
// not the once between lifecycle-aware components (see UseLifecycle)
func UseLateInit(lateInitFn func()) {
	if lateInitFn == nil {
		panic(newErrNilLateInit())
	}
	addLateInitImpl(func() error {
		lateInitFn()
		return nil
	}, 1)
}

// UseLateInitE can be called inside `provide` callback to register a `lateInitFn`
// that will be called after `provider` function returns but before `getComponent` returns.
// See UseLateInit for details.
//
// Unlike UseLateInit, `lateInitFn` can return a non-nil error that will be:
// - thrown in panic on Provide's GetComponent() call
// - returned as error from ProvideE's GetComponent() call
// If `lateInitFn` returns an error (or panics) the component and all its dependents will not be registered
func UseLateInitE(lateInitFn func() error) {
	if lateInitFn == nil {
		panic(newErrNilLateInit())
	}
	addLateInitImpl(lateInitFn, 1)
}

func addLateInitImpl(
	lateInitFn func() error,
	callerLevel runtm.CallerLevel,
) {
	userCallerCtxs, isFound := globalRegistry.HasKnownParentCallerCtx(callerLevel + 1)
	if !isFound {
		panic(errNotInProviderFn)
	}
	globalRegistry.nodeFrames.LateInitQueuePush(lateInitFn, userCallerCtxs)
}
