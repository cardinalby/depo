package depo

// LifecycleHookInfo represents a lifecycle hook registered for a component using UseLifecycle
type LifecycleHookInfo interface {
	ID() uintptr
	String() string
	ComponentInfo() ComponentInfo
	Tag() any
}

// RunnerListener is an interface for listening to events of LifecycleHooks managed by a Runner
type RunnerListener interface {
	// OnStart is called when the Runner starts a LifecycleHookInfo (corresponds Starter.Start,
	// Runnable.Run, ReadinessRunnable.Run methods)
	OnStart(lcHook LifecycleHookInfo)

	// OnReady is called when the LifecycleHookInfo is ready and doesn't block starting of its dependents anymore.
	// It corresponds to ReadinessRunnable.Run onReady callback or Starter.Start nil result. For Runnable,
	// it's called immediately after OnStart
	OnReady(lcHook LifecycleHookInfo)

	// OnClose is called when the Runner closes a LifecycleHookInfo (corresponds Closer.Close or cancelling context
	// for Runnable and ReadinessRunnable)
	OnClose(lcHook LifecycleHookInfo, cause error)

	// OnDone corresponds receiving a non-nil error from Start() or any result from Runnable.Run or
	// ReadinessRunnable.Run.
	OnDone(lcHook LifecycleHookInfo, result error)

	// OnShutdown is called once when the Runner is shutting down, either due to an error in one of the components
	// (see ErrLifecycleHookFailed) or a shutdown due to context cancellation (returns context.Canceled error)
	OnShutdown(cause error)
}

type runnerListeners []RunnerListener

func (l runnerListeners) OnStart(lcHook LifecycleHookInfo) {
	for _, listener := range l {
		listener.OnStart(lcHook)
	}
}

func (l runnerListeners) OnReady(lcHook LifecycleHookInfo) {
	for _, listener := range l {
		listener.OnReady(lcHook)
	}
}

func (l runnerListeners) OnClose(lcHook LifecycleHookInfo, cause error) {
	for _, listener := range l {
		listener.OnClose(lcHook, cause)
	}
}

func (l runnerListeners) OnDone(lcHook LifecycleHookInfo, result error) {
	for _, listener := range l {
		listener.OnDone(lcHook, result)
	}
}

func (l runnerListeners) OnShutdown(cause error) {
	for _, listener := range l {
		listener.OnShutdown(cause)
	}
}
