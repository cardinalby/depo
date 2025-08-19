package depo

import "github.com/cardinalby/depo/internal/runtm"

// nodeFrame represents a single frame in the components resolving stack
type nodeFrame interface {
	GetDepNode() depNode
	CreateLinkedNodeFrame(next *linkedNodeFrame, nextCallerCtxs runtm.CallerCtxs) *linkedNodeFrame
	GetParentCallerCtxs() runtm.CallerCtxs
}

type nodeProviderFrame struct {
	// depNode that is associated with this frame
	depNode depNode

	// callerCtxs traces the `getComponent` call inside user's `provider` function.
	// If it's a root call (first frame in the nodeFramesStack), it is empty
	callerCtxs runtm.CallerCtxs
}

func (f nodeProviderFrame) GetDepNode() depNode {
	return f.depNode
}

func (f nodeProviderFrame) GetParentCallerCtxs() runtm.CallerCtxs {
	return f.callerCtxs
}

func (f nodeProviderFrame) CreateLinkedNodeFrame(next *linkedNodeFrame, nextCallerCtxs runtm.CallerCtxs) *linkedNodeFrame {
	return &linkedNodeFrame{
		depNode:        f.depNode,
		lateInitSeqNum: 0,
		lateInitRegAt:  nil,
		next:           next,
		nextCallerCtxs: nextCallerCtxs,
	}
}

// frame associated with a LateInit function of a component
type nodeLateInitFrame struct {
	// depNode that is associated with this frame
	depNode depNode

	// sequential number of the lateInit in the nodeFramesStack (for this depNode)
	seqNum int

	// callerCtxs traces lateInit registration call in the client code in `provider` function.
	// The first item is the call to UseLateInit
	regCallerCtxs runtm.CallerCtxs

	// lateInitFn is the function that will be called after unwinding dep frames stack. Is nil in frames
	// stored in errors
	lateInitFn func() error
}

func (f nodeLateInitFrame) GetDepNode() depNode {
	return f.depNode
}

func (f nodeLateInitFrame) GetParentCallerCtxs() runtm.CallerCtxs {
	return nil
}

func (f nodeLateInitFrame) CreateLinkedNodeFrame(next *linkedNodeFrame, nextCallerCtxs runtm.CallerCtxs) *linkedNodeFrame {
	return &linkedNodeFrame{
		depNode:        f.depNode,
		lateInitSeqNum: f.seqNum,
		lateInitRegAt:  f.regCallerCtxs,
		next:           next,
		nextCallerCtxs: nextCallerCtxs,
	}
}
