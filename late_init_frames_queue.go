package depo

import (
	"fmt"
	"slices"

	"github.com/cardinalby/depo/internal/tests"
)

type lateInitFramesQueue struct {
	// emulate slice pointers to reuse capacity from the front after queue is empty.
	// improvement: use circular buffer
	frontIndex int
	items      []nodeLateInitFrame
}

func newLateInitFramesQueue() *lateInitFramesQueue {
	return &lateInitFramesQueue{
		frontIndex: -1,
	}
}

func (q *lateInitFramesQueue) Len() int {
	if q.frontIndex >= 0 {
		return len(q.items) - q.frontIndex
	}
	return 0
}

func (q *lateInitFramesQueue) PopFront() nodeLateInitFrame {
	if q.frontIndex < 0 {
		panic("pop on empty queue")
	}
	node := q.items[q.frontIndex]
	if len(q.items)-q.frontIndex == 1 {
		q.frontIndex = -1
		// clear the slice but keep the capacity
		q.items = q.items[:0]
	} else {
		q.items[q.frontIndex] = nodeLateInitFrame{}
		q.frontIndex++
	}
	return node
}

func (q *lateInitFramesQueue) Push(item nodeLateInitFrame) {
	q.items = append(q.items, item)
	if q.frontIndex < 0 {
		q.frontIndex = 0
	}
}

func (q *lateInitFramesQueue) DeleteNodeFrames(depNode depNode, count int) {
	if q.frontIndex < 0 {
		return
	}
	for i := q.frontIndex; i < len(q.items) && count > 0; i++ {
		if q.items[i].depNode == depNode {
			// improvement: delete range
			q.items = slices.Delete(q.items, i, i+1)
			i-- // adjust index after deletion
			count--
		}
	}
	if len(q.items) == 0 {
		q.frontIndex = -1
	}
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && count != 0 {
		panic(fmt.Sprintf(
			"stats for %s has queuedLateInitsCount %d != 0",
			depNode.GetDepInfo().String(),
			count,
		))
	}
}

func (q *lateInitFramesQueue) CleanUp() {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && q.frontIndex >= 0 {
		panic("queue is not empty")
	}
	q.items = nil
	q.frontIndex = -1
}
