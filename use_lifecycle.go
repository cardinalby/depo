package depo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"

	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/tests"
)

var errNotInProviderFn = errors.New("must be called inside `provider` function")

// LifecycleHookBuilder is used for building a component's lifecycle behavior
type LifecycleHookBuilder interface {
	// AddReadinessRunnable registers a ReadinessRunnable in the component's lifecycle.
	// Panics if any other lifecycle methods (e.g. AddRunnable, AddStarter, AddCloser) have already been called.
	// `options` can be used to configure the way it's handled by the Runner:
	// OptStartTimeout, OptNilRunResultAsError
	AddReadinessRunnable(
		readinessRunnable ReadinessRunnable,
		options ...ReadinessRunnableOption,
	) LifecycleHookBuilder

	// AddReadinessRunFn is a convenience method to AddReadinessRunFn having only its Run function
	AddReadinessRunFn(
		runFn func(ctx context.Context, onReady func()) error,
		options ...ReadinessRunnableOption,
	) LifecycleHookBuilder

	// AddRunnable registers a Runnable in the component's lifecycle.
	// Panics if any other lifecycle methods (e.g. AddReadinessRunnable, AddStarter, AddCloser) have already been called.
	// `options` can be used to configure the way it's handled by the Runner: OptNilRunResultAsError
	AddRunnable(
		runnable Runnable,
		options ...RunnableOption,
	) LifecycleHookBuilder

	// AddRunFn is a convenience method to AddRunnable having only its Run function
	AddRunFn(
		runFn func(ctx context.Context) error,
		options ...RunnableOption,
	) LifecycleHookBuilder

	// AddStarter registers a Starter in the component's lifecycle.
	// Panics if any runnables or starters has been already added in the chain
	// `options` can be used to configure the way it's handled by the Runner: OptStartTimeout
	AddStarter(
		starter Starter,
		options ...StarterOption,
	) LifecycleHookBuilder

	// AddStartFn is a convenience method to AddStarter having only its Start function
	AddStartFn(
		startFn func(ctx context.Context) error,
		options ...StarterOption,
	) LifecycleHookBuilder

	// AddCloser registers a Closer in the component's lifecycle
	// Panics if any runnables or closers has been already added in the chain
	AddCloser(closer Closer) LifecycleHookBuilder

	// AddCloseFn is a convenience method to AddCloser having only its Close function
	AddCloseFn(closeFn func()) LifecycleHookBuilder

	// Tag sets a tag that can be observed in RunnerListener calls
	Tag(tag any) LifecycleHookBuilder
}

// UseLifecycle should be called inside `provider` callback to set up currently providing component's
// lifecycle behavior using LifecycleHookBuilder methods.
// You can call UseLifecycle multiple times inside one `provide` function adding multiple lifecycle hooks,
// they will act as independent instances that have the same set of dependencies and dependents.
// Note that using two separate UseLifecycle with Add... calls is not equivalent to using
// one UseLifecycle with multiple Add... calls
func UseLifecycle() LifecycleHookBuilder {
	userCallerCtxs, isFound := globalRegistry.HasKnownParentCallerCtx(1)
	if !isFound {
		panic(fmt.Errorf("UseLifecycle %w", errNotInProviderFn))
	}
	hookBuilder := &lifecycleHookBuilder{
		regAt: userCallerCtxs,
	}
	globalRegistry.AddCurrentlyResolvingNodeLifecycleHook(hookBuilder)
	return hookBuilder
}

type lifecycleHookBuilder struct {
	regAt                runtm.CallerCtxs
	readinessRunnable    ReadinessRunnable
	readinessRunnableCfg readinessRunnableCfg
	runnable             Runnable
	runnableCfg          runnableCfg
	starter              Starter
	starterCfg           starterCfg
	closer               Closer
	tag                  any
}

var errAlreadyAdded = errors.New("already added")

func (b *lifecycleHookBuilder) AddReadinessRunnable(
	readinessRunnable ReadinessRunnable,
	options ...ReadinessRunnableOption,
) LifecycleHookBuilder {
	b.checkCanAddRunnable()
	if readinessRunnable == nil {
		panic(fmt.Errorf("%w readinessRunnable", errNilValue))
	}
	b.readinessRunnable = readinessRunnable
	b.readinessRunnableCfg = newReadinessRunnableCfg(options)
	return b
}

func (b *lifecycleHookBuilder) AddReadinessRunFn(
	readinessRunFn func(ctx context.Context, onReady func()) error,
	options ...ReadinessRunnableOption,
) LifecycleHookBuilder {
	b.checkCanAddRunnable()
	if readinessRunFn == nil {
		panic(fmt.Errorf("%w readinessRunFn", errNilValue))
	}
	b.readinessRunnable = &fnReadinessRunnable{fn: readinessRunFn}
	b.readinessRunnableCfg = newReadinessRunnableCfg(options)
	return b
}

func (b *lifecycleHookBuilder) AddRunnable(
	runnable Runnable,
	options ...RunnableOption,
) LifecycleHookBuilder {
	b.checkCanAddRunnable()
	if runnable == nil {
		panic(fmt.Errorf("%w runnable", errNilValue))
	}
	b.runnable = runnable
	b.runnableCfg = newRunnableCfg(options)
	return b
}

func (b *lifecycleHookBuilder) AddRunFn(
	runFn func(ctx context.Context) error,
	options ...RunnableOption,
) LifecycleHookBuilder {
	b.checkCanAddRunnable()
	if runFn == nil {
		panic(fmt.Errorf("%w runFn", errNilValue))
	}
	b.runnable = &fnRunnable{fn: runFn}
	b.runnableCfg = newRunnableCfg(options)
	return b
}

func (b *lifecycleHookBuilder) AddStarter(
	starter Starter,
	options ...StarterOption,
) LifecycleHookBuilder {
	b.checkNoRunnable()
	if b.starter != nil {
		panic(fmt.Errorf("starter %w", errAlreadyAdded))
	}
	if starter == nil {
		panic(fmt.Errorf("%w starter", errNilValue))
	}
	b.starter = starter
	b.starterCfg = newStarterCfg(options)
	return b
}

func (b *lifecycleHookBuilder) AddStartFn(
	startFn func(ctx context.Context) error,
	options ...StarterOption,
) LifecycleHookBuilder {
	b.checkNoRunnable()
	if b.starter != nil {
		panic(fmt.Errorf("starter %w", errAlreadyAdded))
	}
	if startFn == nil {
		panic(fmt.Errorf("%w startFn", errNilValue))
	}
	b.starter = &fnStarter{fn: startFn}
	b.starterCfg = newStarterCfg(options)
	return b
}

func (b *lifecycleHookBuilder) AddCloser(closer Closer) LifecycleHookBuilder {
	b.checkNoRunnable()
	if b.closer != nil {
		panic(fmt.Errorf("closer %w", errAlreadyAdded))
	}
	if closer == nil {
		panic(fmt.Errorf("%w closer", errNilValue))
	}
	b.closer = closer
	return b
}

func (b *lifecycleHookBuilder) AddCloseFn(
	closeFn func(),
) LifecycleHookBuilder {
	b.checkNoRunnable()
	if b.closer != nil {
		panic(fmt.Errorf("closer %w", errAlreadyAdded))
	}
	if closeFn == nil {
		panic(fmt.Errorf("%w closeFn", errNilValue))
	}
	b.closer = &fnCloser{fn: closeFn}
	return b
}

func (b *lifecycleHookBuilder) Tag(tag any) LifecycleHookBuilder {
	b.tag = tag
	return b
}

func (b *lifecycleHookBuilder) checkCanAddRunnable() {
	b.checkNoRunnable()
	if b.starter != nil {
		panic(fmt.Errorf("starter %w", errAlreadyAdded))
	}
	if b.closer != nil {
		panic(fmt.Errorf("closer %w", errAlreadyAdded))
	}
}

func (b *lifecycleHookBuilder) checkNoRunnable() {
	if b.runnable != nil {
		panic(fmt.Errorf("runnable %w", errAlreadyAdded))
	}
	if b.readinessRunnable != nil {
		panic(fmt.Errorf("readinessRunnable %w", errAlreadyAdded))
	}
}

func (b *lifecycleHookBuilder) getHook() (*lifecycleHook, bool) {
	if b.starter == nil &&
		b.closer == nil &&
		b.runnable == nil &&
		b.readinessRunnable == nil {
		return nil, false
	}

	lcHook := &lifecycleHook{
		regAt: b.regAt,
		tag:   b.tag,
	}
	switch {
	case b.readinessRunnable != nil:
		adapter := newPhasedReadinessRunnable(b.readinessRunnable)
		lcHook.starter = adapter
		lcHook.waiter = adapter
		lcHook.causeCloser = adapter
		lcHook.starterCfg = b.readinessRunnableCfg.starterCfg
		lcHook.waiterCfg = b.readinessRunnableCfg.waiterCfg

	case b.runnable != nil:
		adapter := newPhasedRunnable(b.runnable)
		lcHook.starter = adapter
		lcHook.waiter = adapter
		lcHook.causeCloser = adapter
		lcHook.waiterCfg = b.runnableCfg.waiterCfg

	default:
		lcHook.starter = b.starter
		lcHook.starterCfg = b.starterCfg
		lcHook.closer = b.closer
	}

	return lcHook, true
}

type lifecycleHook struct {
	regAt       runtm.CallerCtxs
	starter     Starter
	starterCfg  starterCfg
	waiter      waiter
	waiterCfg   waiterCfg
	closer      Closer
	causeCloser causeCloser
	tag         any
	isRunning   atomic.Bool
}

func (hook *lifecycleHook) isTrustedAsyncCloser() bool {
	_, isAsync := hook.causeCloser.(trustedAsyncCloser)
	return isAsync
}

func (hook *lifecycleHook) hasCloser() bool {
	return hook.closer != nil || hook.causeCloser != nil
}

func (hook *lifecycleHook) close(cause error) {
	switch {
	case hook.causeCloser != nil:
		hook.causeCloser.close(cause)
	case hook.closer != nil:
		hook.closer.Close()
	default:
		if tests.IsTestingBuild {
			panic("no causeCloser or closer")
		}
	}
}

func (hook *lifecycleHook) String() string {
	strWithImplName, _ := hook.stringForPhase("")
	return strWithImplName
}

func (hook *lifecycleHook) stringForPhase(phase failedLifecyclePhase) (strWithImplName string, methodName string) {
	var sb strings.Builder
	if phase == "" {
		sb.WriteString(hook.getRegRolesString())
	} else {
		var name string
		name, methodName = hook.getPhaseImplStrings(phase)
		sb.WriteString(name)
	}
	if hook.tag != nil {
		sb.WriteString(fmt.Sprintf(" (tag: %v)", hook.tag))
	}
	sb.WriteString(" registered at\n")
	sb.WriteString(hook.regAt.FileLines())
	return sb.String(), methodName
}

func (hook *lifecycleHook) getPhaseImplStrings(phase failedLifecyclePhase) (name string, method string) {
	var impl any
	switch phase {
	case failedLifecyclePhaseStart:
		method = lcStartMethodName
		impl = hook.starter
	case failedLifecyclePhaseWait:
		impl = hook.waiter
		method = lcRunMethodName
	}
	if impl == nil {
		if tests.IsTestingBuild {
			panic(fmt.Sprintf("unexpected phase %s or nil impl", phase))
		}
		return "", method
	}
	if adapterStringer, ok := impl.(lcAdapterStringer); ok {
		return adapterStringer.getLcAdapterImplNameAndMethod()
	}
	return reflect.TypeOf(impl).String(), method
}

func (hook *lifecycleHook) getRegRolesString() string {
	var roles []string
	if hook.starter != nil {
		switch hook.starter.(type) {
		case *phasedRunnable:
			return "Runnable"
		case *phasedReadinessRunnable:
			return "ReadinessRunnable"
		}
		roles = append(roles, "Starter")
	}
	if hook.closer != nil {
		roles = append(roles, "Closer")
	}
	return strings.Join(roles, "/")
}
