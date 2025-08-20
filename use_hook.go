package depo

import (
	"errors"
)

var errNilValue = errors.New("nil")

// UseComponentID allows you to get the component ID inside the Provider function for debugging purposes.
// Returns 0 if called outside a Provider function.
func UseComponentID() uint64 {
	_, isFound := globalRegistry.HasKnownParentCallerCtx(1)
	if !isFound {
		return 0
	}
	return uint64(globalRegistry.GetCurrentlyResolvingNodeDepId())
}

// UseTag allows you to set a custom tag for the current component inside Provider function. The tag can be
// observed later in RunnerListener calls.
// It tags the component, you can also tag a lifecycle hook using UseLifecycle().Tag()
// Panics if called outside a `provider` function.
func UseTag(tag any) {
	_, isFound := globalRegistry.HasKnownParentCallerCtx(1)
	if !isFound {
		panic(errNotInProviderFn)
	}
	globalRegistry.SetCurrentlyResolvingNodeTag(tag)
}
