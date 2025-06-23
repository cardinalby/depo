package depo

import (
	"errors"
	"fmt"

	"github.com/cardinalby/depo/internal/errr"
	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/strs"
	"github.com/cardinalby/depo/internal/tests"
)

// store the lib's errors here just for the case if somebody decides to override the exported
// errors or return them from user code to manipulate the lib's behavior

var errCyclicDependency = errors.New("cyclic dependency")

const cyclicDependencyRecommendation = "Use UseLateInit() to solve"

var errNilProviderFn = errors.New("nil provider function")

type stackAwareError interface {
	error
	tailorErrStackForNode(node depNode, depCallerCtxs runtm.CallerCtxs) (stackAwareError, bool)
	getErrStackStart() *linkedNodeFrame
}

type errNodeFramePanic struct {
	panicValue any
}

func (e errNodeFramePanic) Error() string {
	return fmt.Sprintf("panic: %v", e.panicValue)
}

func (e errNodeFramePanic) Unwrap() error {
	// can contain a library exported error in panicValue
	if err, isErr := e.panicValue.(error); isErr {
		return err
	}
	return nil
}

// errDepRegFailed is an error in a user-defined component initialization making a dependency
// resolution fail
type errDepRegFailed struct {
	// failedFrame is the frame that failed. In case of late init failure it contains a chain of reconstructed frames
	// starting from a frame pointing to the depNode's provider frame and ending with a late init frame
	// that caused the failure
	failedFrame *linkedNodeFrame

	// the reason of the failure. It's one of:
	// - errNodeFramePanic: a panic happened in the user code in `provider` or late init function body
	// - an error returned by a late init function (added by UseLateInitE())
	cause error

	// hasUserCodePanic indicates that cause is errNodeFramePanic (or contains errNodeFramePanic
	// wrapped in late init err) with a panic value thrown by user code, not by the library internals.
	// In this case there are no library exported errors in the cause chain and the whole cause chain looks like:
	// - errNodeFramePanic
	//    - user panicValue OR
	//      errDepRegFailed
	//        - errNodeFramePanic
	//          ... (and so on)
	// We keep this flag at the top level (and inherit it from wrapped errors)
	// to avoid deep unwrapping of the cause chain
	hasUserCodePanic bool

	formatForPanicking bool
}

// newErrDepRegFailedWithPanic creates a new errFramePanic instance from the panic value.
// `callerLevel` is the level of closest user code callerCtxs frame relative to the callerCtxs of this function
// `frameBoundary` is CallerCtx of the library internal function that started the frame
func newErrDepRegFailedWithPanic(
	depFrame nodeFrame,
	panicValue any,
	callerLevel runtm.CallerLevel,
	frameBoundary runtm.CallerCtx,
) errDepRegFailed {
	userCallerCtxs, _, isFound := runtm.FindClosestKnownCallerCtxInCallStack(callerLevel+1, frameBoundary)
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && !isFound {
		panic("newErrDepRegFailedWithPanic: no user frames found in frameBoundary")
	}

	panicValueErr, isErr := panicValue.(error)
	// all lib panic values are errors. If it's not an error, it was thrown by user code
	hasUserCodePanic := !isErr || hasWrappedUserPanic(panicValueErr)

	res := errDepRegFailed{
		failedFrame: depFrame.CreateLinkedNodeFrame(nil, userCallerCtxs),
		cause: errNodeFramePanic{
			panicValue: panicValue,
		},
		hasUserCodePanic: hasUserCodePanic,
	}
	return res
}

func newErrDepRegFailedWithErr(
	frame nodeProviderFrame,
	err error,
) errDepRegFailed {
	return errDepRegFailed{
		failedFrame:      frame.CreateLinkedNodeFrame(nil, nil),
		cause:            err,
		hasUserCodePanic: false,
	}
}

func newErrDepRegFailedWithLateInitErr(
	lateInitFrame nodeLateInitFrame,
	lateInitErr error,
) errDepRegFailed {
	return errDepRegFailed{
		failedFrame:      lateInitFrame.CreateLinkedNodeFrame(nil, nil),
		cause:            lateInitErr,
		hasUserCodePanic: hasWrappedUserPanic(lateInitErr),
	}
}

func (e errDepRegFailed) Empty() bool {
	return e.cause == nil && e.failedFrame == nil
}

func (e errDepRegFailed) Error() string {
	var lines strs.Lines

	var wrappedStackAwareErr stackAwareError
	if !e.hasUserCodePanic {
		var ok bool
		if wrappedStackAwareErr, ok = errr.As[stackAwareError](e.cause); ok {
			if wrappedStackAwareErr, ok = wrappedStackAwareErr.tailorErrStackForNode(
				e.failedFrame.depNode, e.failedFrame.nextCallerCtxs,
			); ok {
				return wrappedStackAwareErr.Error()
			}
		}
	}

	// if it's a user code panic, the chain doesn't contain user-wrapped errors.
	// To make a pretty panic message (starting from the frame where the panic happened and going down the stack)
	// we need to go the way opposite to normal fmt.Errorf: start from the deepest wrapped error and go up
	var next error = e
	isFirstDepRegFailedErr := true
	for {
		//goland:noinspection GoTypeAssertionOnErrors
		if depResolvingPanicErr, ok := next.(errNodeFramePanic); ok {
			if next = depResolvingPanicErr.Unwrap(); next == nil {
				prefix := ""
				if !e.formatForPanicking {
					prefix = "panic: "
				}
				lines.Push(prefix + fmt.Sprintf("%v", depResolvingPanicErr.panicValue))
				break
			}
			continue
		}

		//goland:noinspection GoTypeAssertionOnErrors
		if depRegFailedErr, ok := next.(errDepRegFailed); ok {
			frame := depRegFailedErr.failedFrame
			lnFrameRendering := lnFramesChainStrOptNormal
			if isFirstDepRegFailedErr {
				lnFrameRendering = lnFramesChainStrOptSkipFirstDepInfo
				isFirstDepRegFailedErr = false
			}
			if wrappedStackAwareErr == nil ||
				wrappedStackAwareErr.getErrStackStart().FindNodeFrameInChain(frame.depNode) == nil {
				lines.Push(frame.ChainString(lnFrameRendering))
			}
			next = depRegFailedErr.cause
			continue
		}

		lines.Push(next.Error())
		break
	}

	return lines.JoinReverse()
}

func (e errDepRegFailed) Unwrap() error {
	return e.cause
}

func hasWrappedUserPanic(err error) bool {
	var depRegFailedErr errDepRegFailed
	return errors.As(err, &depRegFailedErr) && depRegFailedErr.hasUserCodePanic
}

func panicIfHasWrappedUserCodePanic(err error) {
	if regErr, ok := errr.As[errDepRegFailed](err); ok && regErr.hasUserCodePanic {
		regErr.formatForPanicking = true
		panic(regErr)
	}
}

type errInProvideContextStruct struct {
	name           string
	userCallerCtxs runtm.CallerCtxs
	nodeFrame      nodeFrame
}

func (e errInProvideContextStruct) Error() string {
	var lines strs.Lines
	lines.Push(e.name + " " + errInProvideContext.Error())
	lines.Push(e.userCallerCtxs.FileLines())
	return lines.Join()
}

func (e errInProvideContextStruct) Unwrap() error {
	return errInProvideContext
}

type errCyclicDependencyStruct struct {
	cycleStartFrame *linkedNodeFrame
}

func (e errCyclicDependencyStruct) Error() string {
	var lines strs.Lines
	lines.Push(errCyclicDependency.Error() + ". " + cyclicDependencyRecommendation)
	lines.Push(e.cycleStartFrame.ChainString(lnFramesChainStrOptCycle))
	return lines.Join()
}

func (e errCyclicDependencyStruct) Unwrap() error {
	return errCyclicDependency
}

func (e errCyclicDependencyStruct) tailorErrStackForNode(node depNode, _ runtm.CallerCtxs) (stackAwareError, bool) {
	if found := e.cycleStartFrame.FindNodeFrameInChain(node); found != nil {
		e.cycleStartFrame = found
		return e, true
	}
	return e, false
}

func (e errCyclicDependencyStruct) getErrStackStart() *linkedNodeFrame {
	return e.cycleStartFrame
}
