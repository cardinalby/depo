package depo

// UseComponentID allows you to get the component ID inside the Provider function for debugging purposes.
// Returns 0 if called outside a Provider function.
func UseComponentID() uint64 {
	_, isFound := globalRegistry.HasKnownParentCallerCtx(1)
	if !isFound {
		return 0
	}
	return uint64(globalRegistry.GetCurrentlyResolvingDepId())
}
