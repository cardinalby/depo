package depo

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/pkg/contexts"
)

// LifecycleHookNode is an extension of LifecycleHookInfo providing information about dependencies between
// lifecycle hooks for debugging purposes.
type LifecycleHookNode interface {
	LifecycleHookInfo
	DependsOnHooks() []LifecycleHookNode
}

// Runner is a lifecycle manager that runs and closes components' lifecycle hooks in the correct order
// according to the dependency graph of the components.
type Runner interface {
	// Run method:
	// 1. Starts all Starter, Runnable or ReadinessRunnable hooks in the proper order (from leaves to roots)
	// 2. Once they are all started successfully, it calls the `onReady` callback
	// 3. If any of the hooks fail to start, it shuts down all hooks that have already been started
	// 4. Waits for:
	//  - the `ctx` to be done (e.g. on `SIGINT`/`SIGTERM`)
	//  - any of the Runnable hooks to return an error from `Run` method
	//  - all the Runnable hooks to return nil from `Run` method
	// 5. Shuts down all Closer, Runnable or ReadinessRunnable hooks in the proper order (from roots to leaves)
	// 6. Once all hooks are done, returns the error that caused the shutdown:
	// - `nil` if all components finished with no error
	// - ErrUnexpectedRunNilResult if OptNilRunResultAsError option is used
	// - ctx.Err() if the ctx was cancelled
	// - ErrLifecycleHookFailed wrapping the original hook's error
	//
	// If the context is nil, a "shutdown context" is used (gets cancelled by SIGINT/SIGTERM).
	// See pkg/contexts/NewShutdownContext for details.
	Run(ctx context.Context, onReady func()) error

	// GetRootLifecycleHookNodes can be used for debugging / logging purposes
	GetRootLifecycleHookNodes() []LifecycleHookNode
}

const newRunnerCtorName = "NewRunnerE"

// NewRunner creates a new Runner instance that manages the lifecycle of component graph with roots
// requested by the provider function. It panics with ErrCyclicDependency if a cyclic dependency is detected
func NewRunner(
	provider func(),
	options ...RunnerOption,
) Runner {
	if provider == nil {
		panic(errNilProviderFn)
	}
	r, err := newRunnerImpl(func() error {
		provider()
		return nil
	}, 1, options...)
	if err != nil {
		panic(err)
	}
	return r
}

// NewRunnerE creates a new Runner instance that manages the lifecycle of component graph with roots
// requested by the provider function. Returns ErrCyclicDependency if a cyclic dependency is detected
func NewRunnerE(
	provider func() error,
	options ...RunnerOption,
) (r Runner, err error) {
	if provider == nil {
		return nil, errNilProviderFn
	}
	return newRunnerImpl(provider, 1, options...)
}

func newRunnerImpl(
	provider func() error,
	callerLevel runtm.CallerLevel,
	options ...RunnerOption,
) (r Runner, err error) {
	userCallerCtxs, isFound := globalRegistry.HasKnownParentCallerCtx(callerLevel + 1)
	if isFound {
		topFrame := globalRegistry.Frames().Stack().Peek()
		return nil, errInProvideContextStruct{
			name:           newRunnerCtorName,
			userCallerCtxs: userCallerCtxs,
			nodeFrame:      topFrame,
		}
	}

	regAt := runtm.NewCallerCtx(1)
	rootNode := newComponentNode(
		func() (struct{}, error) { // provider
			err := provider()
			return struct{}{}, err
		},
		regAt,
		newRunnerCtorName,
	)

	// since we are not in provider context this call starts a new stack and all late inits (if any) will be
	// executed before GetComponent returns
	_, err = rootNode.GetComponent(callerLevel + 1)
	panicIfHasWrappedUserCodePanic(err)
	if err != nil {
		return nil, fmt.Errorf("provider error: %w", err)
	}

	var lcNodesGraph lcNodesGraph
	// if depNode has no runnables, leave lcNodes empty, it will lead to no-op runner
	if rootNode.regState == nodeRegStateWithLcHooks {
		if lcNodesGraph, err = newLcNodesGraph(rootNode); err != nil {
			return nil, err
		}
	}

	return &runner{
		graph:  lcNodesGraph,
		config: newRunnerCfg(options),
	}, nil
}

type runner struct {
	graph     lcNodesGraph
	config    runnerCfg
	isRunning atomic.Bool
}

func (r *runner) Run(ctx context.Context, onReady func()) error {
	if ctx == nil {
		var cancel context.CancelCauseFunc
		ctx, cancel = contexts.NewShutdownContext(context.Background())
		defer cancel(nil)
	}
	if r.isRunning.Swap(true) {
		return errAlreadyRunning
	}
	defer func() {
		r.isRunning.Store(false)
	}()
	rs := newRunnerSession(r.graph, onReady, r.config)
	return rs.run(ctx)
}

func (r *runner) GetRootLifecycleHookNodes() []LifecycleHookNode {
	res := make([]LifecycleHookNode, 0, len(r.graph.roots))
	for _, root := range r.graph.roots {
		res = append(res, root)
	}
	return res
}
