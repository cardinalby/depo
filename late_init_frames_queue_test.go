package depo

import (
	"testing"

	"github.com/cardinalby/depo/internal/dep"
	"github.com/stretchr/testify/require"
)

// stubDepNode implements depNode for testing lateInitFramesQueue
// minimal no-op implementation
var _ depNode = (*stubDepNode)(nil)

type stubDepNode struct {
	id dep.Id
}

func (s *stubDepNode) GetDepInfo() dep.Info {
	return dep.Info{Ctx: dep.Ctx{Id: s.id}}
}

func (s *stubDepNode) IsRegistered() bool {
	return false
}

func (s *stubDepNode) GetRegState() nodeRegState {
	return nodeRegStateNone
}

func (s *stubDepNode) GetLifecycleHooks() []*lifecycleHook {
	return nil
}

func (s *stubDepNode) GetDependsOn() []depNode {
	return nil
}

func (s *stubDepNode) StartProviding() error {
	return nil
}

func (s *stubDepNode) SetRegResult(
	_ error,
	_ []depNode,
	_ []*lifecycleHook,
	_ bool,
) {
}

func (s *stubDepNode) SetTag(_ any) {
}

func (s *stubDepNode) GetProvidedValue() any {
	return nil
}

func TestLateInitFramesQueue_BasicOps(t *testing.T) {
	q := newLateInitFramesQueue()
	require.Equal(t, 0, q.Len())

	nodeA := &stubDepNode{}
	nodeB := &stubDepNode{}

	f1 := nodeLateInitFrame{depNode: nodeA, seqNum: 1}
	f2 := nodeLateInitFrame{depNode: nodeB, seqNum: 2}

	q.Push(f1)
	require.Equal(t, 1, q.Len())
	q.Push(f2)
	require.Equal(t, 2, q.Len())

	// PopFront should return frames in insertion order
	out1 := q.PopFront()
	require.Equal(t, f1, out1)
	require.Equal(t, 1, q.Len())

	out2 := q.PopFront()
	require.Equal(t, f2, out2)
	require.Equal(t, 0, q.Len())
}

func TestLateInitFramesQueue_PopEmpty_Panic(t *testing.T) {
	q := newLateInitFramesQueue()
	require.Panics(t, func() { q.PopFront() })
}

func TestLateInitFramesQueue_DeleteNodeFrames(t *testing.T) {
	q := newLateInitFramesQueue()
	nodeA := &stubDepNode{id: 1}
	nodeB := &stubDepNode{id: 2}

	// delete on empty queue should be no-op
	q.DeleteNodeFrames(nodeA, 1)
	require.Equal(t, 0, q.Len())

	// Push frames: A, B, A
	fA1 := nodeLateInitFrame{depNode: nodeA, seqNum: 1}
	fB := nodeLateInitFrame{depNode: nodeB, seqNum: 1}
	fA2 := nodeLateInitFrame{depNode: nodeA, seqNum: 2}
	q.Push(fA1)
	q.Push(fB)
	q.Push(fA2)
	require.Equal(t, 3, q.Len())

	// Remove both A frames
	q.DeleteNodeFrames(nodeA, 2)
	require.Equal(t, 1, q.Len())
	// remaining should be fB
	r := q.PopFront()
	require.Equal(t, fB, r)
	require.Equal(t, 0, q.Len())
}

func TestLateInitFramesQueue_DeleteNodeFrames_Insufficient_Panic(t *testing.T) {
	q := newLateInitFramesQueue()
	nodeA := &stubDepNode{}

	q.Push(nodeLateInitFrame{depNode: nodeA, seqNum: 1})
	require.Equal(t, 1, q.Len())

	// attempt to delete more frames than exist should panic in testing build
	require.Panics(t, func() { q.DeleteNodeFrames(nodeA, 2) })
}

func TestLateInitFramesQueue_CleanUp(t *testing.T) {
	q := newLateInitFramesQueue()
	// CleanUp on empty queue should not panic
	require.NotPanics(t, func() { q.CleanUp() })
	// Len unaffected
	require.Equal(t, 0, q.Len())

	// Push one frame, CleanUp should panic in testing build
	q.Push(nodeLateInitFrame{depNode: &stubDepNode{}, seqNum: 1})
	require.Panics(t, func() { q.CleanUp() })
}
