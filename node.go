package depo

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/cardinalby/depo/internal/dep"
	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/tests"
)

// nodeProvidingState represents the state of a componentNode during its providing phase
type nodeProvidingState uint8

const (
	// `provider` hasn't been called yet
	nodeProvidingStateNone nodeProvidingState = iota
	// `provider` is being called, the componentNode is in the process of providing
	nodeProvidingStateInProgress
	// `provider` call has finished
	nodeProvidingStateProvided
)

type nodeRegState uint8

const (
	nodeRegStateNone nodeRegState = iota
	nodeRegStateWithLcHooks
	nodeRegStateWithNoLcHooks
)

// for testing purposes, keep track of created nodes
var createdNodes map[dep.Id]depNode
var createdNodesMu *sync.Mutex

func init() {
	if tests.IsTestingBuild {
		createdNodes = make(map[dep.Id]depNode)
		createdNodesMu = &sync.Mutex{}
	}
}

// depNode is an interface that represents a dependency in the graph where
// generic type of componentNode is not needed
type depNode interface {
	GetDepInfo() dep.Info

	// IsRegistered is safe for concurrent access
	IsRegistered() bool
	GetRegState() nodeRegState
	GetLifecycleHooks() []*lifecycleHook
	// GetDependsOn returns a slice of depNodes that this depNode depends on. The order of element is random
	GetDependsOn() []depNode

	StartProviding() error

	SetRegResult(
		regErr error,
		dependsOn []depNode,
		lcHooks []*lifecycleHook,
		hasOwnOrTransitiveRunnables bool,
	)
	SetTag(tag any)

	// GetProvidedValue should be called only after the node is registered and providing is finished
	GetProvidedValue() any
}

// componentNode contains information about a component
type componentNode[T any] struct {
	depInfo dep.Info

	// it can be accessed outside emptyStackSignal sync point in registry
	isRegistered atomic.Bool
	regState     nodeRegState

	lcHooks []*lifecycleHook

	providingState nodeProvidingState
	providedValue  T
	// providedError is the error that occurred during providing the component or in late inits
	providedError error

	// provide is the higher-order wrapper function that is responsible for providing the component.
	// Gets reset after first call
	provider func() (providedValue T, err error)

	dependsOn []depNode
}

var _ depNode = (*componentNode[any])(nil)

func newComponentNode[T any](
	provider func() (providedValue T, err error),
	regAt runtm.CallerCtx,
	ctorName string,
) *componentNode[T] {
	node := &componentNode[T]{
		depInfo: dep.NewDepInfo(
			globalRegistry.NewDepCtx(regAt),
			reflect.TypeOf((*T)(nil)).Elem(),
			ctorName,
		),
		provider: provider,
	}

	if tests.IsTestingBuild {
		createdNodesMu.Lock()
		createdNodes[node.depInfo.Id] = node
		createdNodesMu.Unlock()
	}

	return node
}

func (node *componentNode[T]) GetDepInfo() dep.Info {
	return node.depInfo
}

func (node *componentNode[T]) IsRegistered() bool {
	return node.isRegistered.Load()
}

func (node *componentNode[T]) GetRegState() nodeRegState {
	return node.regState
}

func (node *componentNode[T]) GetLifecycleHooks() []*lifecycleHook {
	return node.lcHooks
}

func (node *componentNode[T]) GetDependsOn() []depNode {
	return node.dependsOn
}

// GetComponent is the lazy singleton wrapper of `provider` that tracks component's dependencies
// Returns:
// - value: the provided value of type T
// - err: an error that occurred during providing the component:
//   - errCyclicDependency if a cyclic dependency was detected
//   - any other error returned by `provider` function
//   - errDepRegFailed in case of panic or failed late init function
func (node *componentNode[T]) GetComponent(clientCallerLevel runtm.CallerLevel) (value T, err error) {
	dependent, userCallerCtxs := globalRegistry.OnGetComponent(clientCallerLevel+1, node)
	return node.providedValue, node.tailorProvidedErrForDependent(dependent, userCallerCtxs)
}

func (node *componentNode[T]) GetProvidedValue() any {
	return node.providedValue
}

// StartProviding calls def.provider() and returns the provided value and error. Subsequent calls will return
// the same memoized value and error.
// Additionally, it can return errCyclicDependency if a cyclic dependency is detected
// (it was called while the component is being provided).
func (node *componentNode[T]) StartProviding() (err error) {
	// if we got here, it means we are the only goroutine resolving the dependency chain
	switch node.providingState {
	case nodeProvidingStateInProgress:
		// somebody requested this component in the chain (in def context) while we are providing it.
		// It can't be a concurrent call outside on def context chain since all that calls are
		// waiting to enter globalRegistry.OnGetComponent()
		// Also def.providedError can't be set at that point since we are in the last dep frame
		// (in most nested call)

		//goland:noinspection GoBoolExpressions
		if tests.IsTestingBuild && node.providedError != nil {
			panic(fmt.Sprintf(
				"node.providedError is already set to %v, but node.providingState is %v",
				node.providedError,
				node.providingState,
			))
		}
		node.providedError = node.createCyclicDependencyError(globalRegistry.nodeFrames.Stack())
		// memoize the error and return it on subsequent calls since the component hasn't been
		// successfully provided
		node.providingState = nodeProvidingStateProvided

	case nodeProvidingStateNone:
		node.providingState = nodeProvidingStateInProgress
		defer func() {
			node.providingState = nodeProvidingStateProvided
		}()
		// this way we can override node.providedError from the nested call of StartProviding() that
		// returned errCyclicDependency error. Allow doing that:
		// if in "A -> B -> A(2)" chain "B" decided to ignore errCyclicDependency from "A(2)" (and don't use the
		// failed component), it can return nil error indicating that it's functional without A. In this case
		// nothing prevents other components from using A in the future (since it doesn't have a cyclic dependency
		// anymore)
		node.providedValue, node.providedError = node.provider()
		node.provider = nil // no longer needed
	default:
	}
	return node.providedError
}

func (node *componentNode[T]) SetRegResult(
	regErr error,
	dependsOn []depNode,
	lcHooks []*lifecycleHook,
	hasOwnOrTransitiveRunnables bool,
) {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && node.isRegistered.Load() {
		panic(fmt.Sprintf("node %s is already registered", node.GetDepInfo().String()))
	}
	if regErr != nil && node.providedError == nil {
		node.providedError = regErr
		// node.providedError is already set in case of cyclic dependency (by 2nd frame) or because of
		// user error returned from `provider` function. It can't be an own late init error since all
		// own late init frames should be removed by the registry once `provider` returns an error.
		//
		// improvement: join the errors if a new error came. Remember that errors.Is() doesn't work
		// with lib "struct" errors as a target since they are not comparable (contain slices).
		// Also, we report the first regErr once a node is failed and remove it from pendingNodes. In case
		// of dependencies` late init errors, we will not able to find the current node since it
		// will be already removed with own error.
		//
		// Ignore regErr for now, it should be enough in the most cases to just have one error to
		// mark node as failed
	}
	node.dependsOn = dependsOn
	node.lcHooks = lcHooks
	if hasOwnOrTransitiveRunnables {
		node.regState = nodeRegStateWithLcHooks
	} else {
		node.regState = nodeRegStateWithNoLcHooks
	}
	node.isRegistered.Store(true)
}

func (node *componentNode[T]) SetTag(tag any) {
	node.depInfo.SetTag(tag)
}

func (node *componentNode[T]) createCyclicDependencyError(stack readonlyNodeFramesStack) *errCyclicDependencyStruct {
	var firstLnFrame *linkedNodeFrame
	var lnFrame *linkedNodeFrame
	var nextCallerCtxs runtm.CallerCtxs

	stack.IterateFromTop()(func(frame nodeFrame) bool {
		if lnFrame == nil {
			// always take the top frame (own provider call), stop at the next frame that belongs to the same node
			lnFrame = frame.CreateLinkedNodeFrame(nil, nil)
			firstLnFrame = lnFrame
		} else {
			if frame.GetDepNode() == node {
				// got a complete cycle, connect the first frame to the current one
				firstLnFrame.next = lnFrame
				firstLnFrame.nextCallerCtxs = nextCallerCtxs
				return false
			}
			lnFrame = frame.CreateLinkedNodeFrame(lnFrame, nextCallerCtxs)
		}
		nextCallerCtxs = frame.GetParentCallerCtxs()
		return true
	})
	return &errCyclicDependencyStruct{
		cycleStartFrame: firstLnFrame,
	}
}

func (node *componentNode[T]) tailorProvidedErrForDependent(dependent depNode, userCallerCtxs runtm.CallerCtxs) error {
	if dependent == nil {
		return node.providedError
	}
	// it's not possible to handle errors wrapped with fmt.Errorf (or other ways in general) because it
	// caches error message generated upon creation and modification of wrapped errors doesn't change the message
	//goland:noinspection GoTypeAssertionOnErrors
	if stackAwareErr, ok := node.providedError.(stackAwareError); ok {
		if err, ok := stackAwareErr.tailorErrStackForNode(dependent, userCallerCtxs); ok {
			return err
		}
	}
	return node.providedError
}
