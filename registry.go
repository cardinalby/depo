package depo

import (
	"fmt"
	"sync/atomic"

	"github.com/cardinalby/depo/internal/dep"
	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/tests"
)

var globalRegistry = newRegistry()

// registry orchestrates the exclusive component 'provide' chains, managing the dep frames stack,
// tracking component dependencies, and handling late initialization.
type registry struct {
	depSerialNoCounter atomic.Uint64
	nodeFrames         *nodeFrames
	// emptyStackSignal is used to synchronize access to the provide context: only one "root"
	// provide chain may proceed at a time. A value in this (buffered) channel indicates that
	// no root context is active and new provide/Provide/ProvideE chains may begin.
	emptyStackSignal chan struct{}
	// pendingNodes tracks relationships between components and their runnables.
	pendingNodes pendingNodes

	// all user-code provider calls have this parent CallerCtx in the stack
	providerParentCallerCtx runtm.CallerCtx

	// all late-init function calls have this parent CallerCtx in the stack
	lateInitParentCallerCtx runtm.CallerCtx
}

func newRegistry() *registry {
	reg := &registry{
		nodeFrames:       newNodeFrames(),
		pendingNodes:     make(pendingNodes),
		emptyStackSignal: make(chan struct{}, 1),
	}
	reg.emptyStackSignal <- struct{}{} // signal that the stack is empty
	// initialize reg.providerParentCallerCtx and reg.lateInitParentCallerCtx doing the first
	// fake calls to the corresponding methods.
	// It will prevent concurrent these fields updates/reads in different goroutines in the
	// future calls
	reg.OnGetComponent(runtm.IsNotInCallStackCallerLevel, nil)
	reg.executeQueuedLateInit()

	return reg
}

func (reg *registry) NewDepCtx(regAt runtm.CallerCtx) dep.Ctx {
	return dep.Ctx{
		Id:    dep.Id(reg.depSerialNoCounter.Add(1)),
		RegAt: regAt,
	}
}

func (reg *registry) HasKnownParentCallerCtx(userCallerLevel runtm.CallerLevel) (
	userCallerCtxs runtm.CallerCtxs,
	isFound bool,
) {
	userCallerCtxs, _, isFound = runtm.FindClosestKnownCallerCtxInCallStack(
		userCallerLevel+1,
		reg.providerParentCallerCtx,
		reg.lateInitParentCallerCtx,
	)
	return userCallerCtxs, isFound
}

func (reg *registry) AddCurrentlyResolvingNodeLifecycleHook(lcHookBuilder *lifecycleHookBuilder) {
	node := reg.nodeFrames.Stack().Peek().GetDepNode()
	reg.pendingNodes.AddLifecycleHookBuilder(node, lcHookBuilder)
}

func (reg *registry) GetCurrentlyResolvingNodeDepId() dep.Id {
	if reg.nodeFrames.Stack().Len() == 0 {
		return dep.Id(0) // no current dep
	}
	return reg.nodeFrames.Stack().Peek().GetDepNode().GetDepInfo().Id
}

func (reg *registry) SetCurrentlyResolvingNodeTag(tag any) {
	reg.nodeFrames.Stack().Peek().GetDepNode().SetTag(tag)
}

// OnGetComponent calls `node.StartProviding()` tracking dependencies between components by maintaining
// "dep frames stack".
// If OnGetComponent is called inside another `OnGetComponent()`, it will proceed immediately
// (it happens in the same single "provider context")
// If OnGetComponent is called outside any "provider/lateInit context", it will wait until the
// "dep frames stack" is empty. Only one root "node.StartProviding()" can be active at a time.
// `clientCallerLevel` is the level of the user code relative to this function call.
// Passing a correct value helps to skip irrelevant frames to optimize the search.
// It returns `dependent` node that requests the current component (if any) and `userCallerCtxs` describing
// lines in user code of dependent's provider that called the current component.
//
// node will also receive SetRegResult() call before the stack becomes empty.
func (reg *registry) OnGetComponent(
	clientCallerLevel runtm.CallerLevel,
	node depNode,
) (
	dependent depNode,
	userCallerCtxs runtm.CallerCtxs,
) {
	if reg.providerParentCallerCtx.Empty() {
		// happens once at the registry init
		reg.providerParentCallerCtx = runtm.NewCallerCtx(0)
		return nil, nil
	}

	userCallerCtxs, isParentFound := reg.HasKnownParentCallerCtx(clientCallerLevel + 1)

	// sync point
	if !isParentFound {
		// safe to call concurrently
		if node.IsRegistered() {
			// shortcut, avoid sync and stack operations.
			// It's a root call and the node is registered which means:
			//	- there are no dependents to track
			//	- it will immediately return a memoized provided value + error
			return nil, nil
		}
		// wait until the nodeFrames is empty (all ongoing provide calls are finished)
		// to initiate a new stack
		<-reg.emptyStackSignal

		// re-check after we acquired stack access (could have been registered in the meantime)
		if node.IsRegistered() {
			reg.notifyStackIsEmpty() // let other goroutines enter
			return nil, nil
		}
	} else {
		// stack should not be empty here, we are in the middle of a nodeFrames
		dependent = reg.nodeFrames.Stack().Peek().GetDepNode()
		if node.GetRegState() == nodeRegStateWithNoLcHooks {
			// shortcut, avoid stack operations. No need to track dependencies since it is already
			// provided/registered and has no reachable runnables.
			// Inform the node about
			return dependent, userCallerCtxs
		}
	}

	if reg.nodeFrames.Stack().Len() > 0 {
		dependent = reg.nodeFrames.Stack().Peek().GetDepNode()
	}
	reg.pendingNodes.Add(dependent, node, userCallerCtxs)

	providerFrame := reg.nodeFrames.PushProviderFrame(node, userCallerCtxs)
	var providingErr error
	hasPanic := true
	// use defer to catch panics
	defer func() {
		if hasPanic {
			panicValue := recover()
			providingErr = newErrDepRegFailedWithPanic(
				providerFrame,
				panicValue,
				runtm.PanicRecoveryDeferFnCallerLevel,
				reg.providerParentCallerCtx,
			)
		}
		popRes := reg.nodeFrames.PopTopFrame(providingErr)
		if providingErr == nil {
			if popRes.isLastNodeFrame {
				reg.pendingNodes.OnLastNodeFramePoppedWithNoErr(popRes.frame.GetDepNode())
			}
		} else {
			reg.pendingNodes.OnDepFrameErr(popRes.frame.GetDepNode(), providingErr, hasPanic)
		}

		isLastStackFrame := reg.nodeFrames.Stack().Len() == 0
		if isLastStackFrame {
			reg.executeLateInits()
		}
		if reg.nodeFrames.IsEmpty() {
			// signal (only after we performed all operations including executeLateInits())
			// that nodeFrames can be accessed by other goroutines
			reg.notifyStackIsEmpty()
		}
	}()

	if providingErr = node.StartProviding(); providingErr != nil {
		providingErr = newErrDepRegFailedWithErr(providerFrame, providingErr)
	}
	hasPanic = false
	return dependent, userCallerCtxs
}

func (reg *registry) executeLateInits() {
	for reg.nodeFrames.LateInitsQueueLen() > 0 {
		reg.executeQueuedLateInit()
	}
}

func (reg *registry) executeQueuedLateInit() {
	if reg.lateInitParentCallerCtx.Empty() {
		// happens once on init
		reg.lateInitParentCallerCtx = runtm.NewCallerCtx(0)
		return
	}

	frame := reg.nodeFrames.PushQueuedLateInitFrameToStack()
	var depRegFailedErr errDepRegFailed
	hasPanic := true

	defer func() {
		// no longer needed, free memory since it's probably a closure capturing
		// a component it initializes that is not needed anymore
		frame.lateInitFn = nil
		if hasPanic {
			panicValue := recover()
			depRegFailedErr = newErrDepRegFailedWithPanic(
				frame,
				panicValue,
				runtm.PanicRecoveryDeferFnCallerLevel,
				reg.lateInitParentCallerCtx,
			)
		}

		var effectiveErr error
		if !depRegFailedErr.Empty() {
			effectiveErr = depRegFailedErr
		}
		popRes := reg.nodeFrames.PopTopFrame(effectiveErr)
		if popRes.isLastNodeFrame {
			if effectiveErr != nil {
				reg.pendingNodes.OnDepLateInitFrameErr(depRegFailedErr)
			} else {
				reg.pendingNodes.OnLastNodeFramePoppedWithNoErr(popRes.frame.GetDepNode())
			}
		}
	}()

	if err := frame.lateInitFn(); err != nil {
		depRegFailedErr = newErrDepRegFailedWithLateInitErr(frame, err)
	}
	hasPanic = false
	return
}

func (reg *registry) Frames() readonlyNodeFrames {
	return reg.nodeFrames
}

func (reg *registry) notifyStackIsEmpty() {
	// signal that the nodeFrames is empty. The channel is buffered, so it won't block.
	// Check it in testing mode
	if tests.IsTestingBuild {
		select {
		case reg.emptyStackSignal <- struct{}{}:
		default:
			panic(fmt.Sprintf("emptyStackSignal has a value"))
		}
	} else {
		reg.emptyStackSignal <- struct{}{}
	}
}
