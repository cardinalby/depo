package depo

// LifecycleHook represents a lifecycle hook registered for a component using UseLifecycle
type LifecycleHook interface {
	ID() uintptr
	String() string
	ComponentInfo() ComponentInfo
	Tag() any
}

// RunnerListener is an interface for listening to events of LifecycleHooks managed by a Runner
type RunnerListener interface {
	// OnStart is called when the Runner starts a LifecycleHook (corresponds Starter.Start,
	// Runnable.Run, ReadinessRunnable.Run methods)
	OnStart(lcHook LifecycleHook)

	// OnReady is called when the LifecycleHook is ready and doesn't block starting of its dependents anymore.
	// It corresponds to ReadinessRunnable.Run onReady callback or Starter.Start nil result. For Runnable,
	// it's called immediately after OnStart
	OnReady(lcHook LifecycleHook)

	// OnClose is called when the Runner closes a LifecycleHook (corresponds Closer.Close or cancelling context
	// for Runnable and ReadinessRunnable)
	OnClose(lcHook LifecycleHook, cause error)

	// OnDone corresponds receiving a non-nil error from Start() or any result from Runnable.Run or
	// ReadinessRunnable.Run.
	OnDone(lcHook LifecycleHook, result error)

	// OnShutdown is called once when the Runner is shutting down, either due to an error in one of the components
	// (see ErrLifecycleHookFailed) or a shutdown due to context cancellation (returns context.Canceled error)
	OnShutdown(cause error)
}

type runnerListeners []RunnerListener

func (l runnerListeners) OnStart(lcHook LifecycleHook) {
	for _, listener := range l {
		listener.OnStart(lcHook)
	}
}

func (l runnerListeners) OnReady(lcHook LifecycleHook) {
	for _, listener := range l {
		listener.OnReady(lcHook)
	}
}

func (l runnerListeners) OnClose(lcHook LifecycleHook, cause error) {
	for _, listener := range l {
		listener.OnClose(lcHook, cause)
	}
}

func (l runnerListeners) OnDone(lcHook LifecycleHook, result error) {
	for _, listener := range l {
		listener.OnDone(lcHook, result)
	}
}

func (l runnerListeners) OnShutdown(cause error) {
	for _, listener := range l {
		listener.OnShutdown(cause)
	}
}
