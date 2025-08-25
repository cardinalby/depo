package depo

import (
	"github.com/cardinalby/depo/internal/errr"
	"github.com/cardinalby/depo/internal/runtm"
)

// ErrCyclicDependency is thrown in panic from provide function (returned by Provide, ProvideE)
// when a cyclic dependency is detected in `provide` function calls (when components call each other
// in a cycle). UseLateInit can be used to resolve cyclic dependencies in this case.
// Also, it's returned from NewRunnerE call if cyclic dependency between lifecycle-aware components is detected
// (proper start/shutdown order cannot be determined)
var ErrCyclicDependency = errCyclicDependency

// Provide defines a component and creates a lazy singleton wrapper for `provide` function (that is responsible for
// constructing a component).
// Once called, `getComponent` tracks calls to other `getComponent` functions internally building the dependency graph.
// In case of cyclic dependencies, it panics with ErrCyclicDependency.
func Provide[T any](
	provider func() (value T),
) (
	getComponent func() T,
) {
	if provider == nil {
		panic(errNilProviderFn)
	}
	node := newComponentNode(
		func() (
			providedValue T,
			err error,
		) {
			return provider(), nil
		},
		runtm.NewCallerCtx(1),
		"Provide",
	)
	return func() T {
		value, err := node.GetComponent(1)
		if err != nil {
			if depRegFailedErr, ok := errr.As[errDepRegFailed](err); ok {
				depRegFailedErr.formatForPanicking = true
				panic(depRegFailedErr)
			}
			panic(err)
		}
		return value
	}
}

// ProvideE defines a component and creates a lazy singleton wrapper for `provide` function (that is responsible for
// constructing a component). It supports returning an error from the `provide` function to avoid panics.
// Once called, `getComponent` tracks calls to other `getComponent` functions internally building
// the dependency graph.
// If it is called inside another `provide` function and returns error, the connection in the graph will not be
// established, and the error can be ignored by the caller if it can do without this dependency.
// In case of cyclic dependencies, it returns ErrCyclicDependency
func ProvideE[T any](
	provider func() (value T, err error),
) (
	getComponent func() (T, error),
) {
	if provider == nil {
		panic(errNilProviderFn)
	}
	node := newComponentNode(provider, runtm.NewCallerCtx(1), "ProvideE")
	return func() (T, error) {
		value, err := node.GetComponent(1)

		panicIfHasWrappedUserCodePanic(err)
		return value, err
	}
}
