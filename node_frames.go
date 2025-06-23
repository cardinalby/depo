package depo

import (
	"fmt"

	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/tests"
)

type depNodeFramesStats struct {
	// total count of frames (in lateInits queue + stack) that are associated with the node.
	totalFramesCount     int
	queuedLateInitsCount int
	// total count of lateInits that has been registered for the node (including already executed)
	registeredLateInitsCount int
}

type nodeFramePopResult struct {
	frame           nodeFrame
	isLastNodeFrame bool
}

type readonlyNodeFrames interface {
	Stack() readonlyNodeFramesStack
	LateInitsQueueLen() int
	IsEmpty() bool
}

type nodeFrames struct {
	stack          *nodeFramesStack
	lateInitsQueue *lateInitFramesQueue
	stats          map[depNode]depNodeFramesStats
}

func newNodeFrames() *nodeFrames {
	return &nodeFrames{
		stack:          newNodeFramesStack(),
		lateInitsQueue: newLateInitFramesQueue(),
		stats:          make(map[depNode]depNodeFramesStats),
	}
}

func (s *nodeFrames) IsEmpty() bool {
	return s.stack.Len() == 0 && s.lateInitsQueue.Len() == 0
}

func (s *nodeFrames) LateInitsQueueLen() int {
	return s.lateInitsQueue.Len()
}

func (s *nodeFrames) Stack() readonlyNodeFramesStack {
	return s.stack
}

func (s *nodeFrames) PushProviderFrame(
	depNode depNode,
	userCallerCtxs runtm.CallerCtxs,
) (frame nodeProviderFrame) {
	stats := s.stats[depNode]
	frame = nodeProviderFrame{
		depNode:    depNode,
		callerCtxs: userCallerCtxs,
	}
	s.stack.Push(frame)
	stats.totalFramesCount++
	s.stats[depNode] = stats

	return frame
}

// LateInitQueuePush schedules `lateInitFn` associated with the current stack top frame.
// the callerCtxs should ensure it is in provider context and the stack is not empty
func (s *nodeFrames) LateInitQueuePush(
	lateInitFn func() error,
	regCallerCtxs runtm.CallerCtxs,
) {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && s.stack.Len() == 0 {
		panic("nodeFrames is empty, but trying to push late init traits")
	}
	topFrameNode := s.stack.Peek().GetDepNode()
	stats, ok := s.stats[topFrameNode]
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && !ok {
		panic(fmt.Sprintf("nodeFrames does not have node %s", topFrameNode.GetDepInfo().String()))
	}
	stats.registeredLateInitsCount++
	stats.queuedLateInitsCount++
	stats.totalFramesCount++
	s.stats[topFrameNode] = stats

	s.lateInitsQueue.Push(nodeLateInitFrame{
		depNode:       topFrameNode,
		seqNum:        stats.registeredLateInitsCount,
		regCallerCtxs: regCallerCtxs,
		lateInitFn:    lateInitFn,
	})
}

func (s *nodeFrames) PopTopFrame(frameErr error) (res nodeFramePopResult) {
	res.frame = s.stack.Pop()
	topDepNode := res.frame.GetDepNode()
	stats, ok := s.stats[topDepNode]
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && !ok {
		panic(fmt.Sprintf("nodeFrames does not have DepId %s", topDepNode.GetDepInfo().Id.String()))
	}

	stats.totalFramesCount--

	if frameErr != nil {
		// an error occurred during providing the component or executing lateInit, no more lateInits
		// should be executed for this node
		s.lateInitsQueue.DeleteNodeFrames(topDepNode, stats.queuedLateInitsCount)
		stats.totalFramesCount -= stats.queuedLateInitsCount
		stats.queuedLateInitsCount = 0
	}

	if stats.totalFramesCount == 0 {
		// no more lateInit frames will be added if we popped the top component frame, can remove dep stats
		delete(s.stats, topDepNode)
		res.isLastNodeFrame = true
	} else {
		// has more frames (or scheduled lateInits) for this DepId, update and keep stats
		s.stats[topDepNode] = stats
	}

	if s.lateInitsQueue.Len() == 0 && s.stack.Len() == 0 {
		// most probably it will not be used again (after the call to root dep),
		// free up allocated capacities of underlying slices
		s.cleanUp()
	}

	return res
}

func (s *nodeFrames) PushQueuedLateInitFrameToStack() nodeLateInitFrame {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild {
		if s.stack.Len() > 0 {
			panic("PushQueuedLateInitFrameToStack should be called when stack is empty")
		}
		if s.lateInitsQueue.Len() == 0 {
			panic("PushQueuedLateInitFrameToStack should be called when queue is not empty")
		}
	}

	frame := s.lateInitsQueue.PopFront()
	s.stack.Push(frame)
	stats := s.stats[frame.depNode]
	stats.queuedLateInitsCount--
	s.stats[frame.depNode] = stats

	return frame
}

func (s *nodeFrames) cleanUp() {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && len(s.stats) != 0 {
		panic(fmt.Sprintf(
			"nodeFrames has len(stats) %d != 0",
			len(s.stats),
		))
	}
	s.stack.CleanUp()
	s.lateInitsQueue.CleanUp()
}
