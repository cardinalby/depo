package depo

// waiter is a component that should be waited to complete its job before its dependencies can be closed
// and can inform Runner about the component's operation completion:
//   - returns non-nil error if it failed and needs to trigger shutdown
//   - returns nil error if it finished successfully and doesn't want to trigger shutdown but can be
//     closed once it has no active dependents
type waiter interface {
	// wait should:
	// - block until the component finishes its job
	// - return nil error if the component finishes successfully and doesn't want to trigger shutdown
	// - return non-nil error if it's also a Closer and Close() has been called, OR if it failed and
	//   needs to trigger shutdown
	wait() error
}

// causeCloser is a Closer that can receive the cause of the shutdown
type causeCloser interface {
	// Close behaves same as Closer.Close() but also accepts a `cause` describing the reason for the shutdown.
	// It's nil if the component is stopped without an error
	close(cause error)
}
