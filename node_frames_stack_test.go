package depo

import (
	"testing"

	"github.com/cardinalby/depo/internal/runtm"
	"github.com/stretchr/testify/require"
)

// mock nodeFrame for stack test
var _ nodeFrame = (*mockNodeFrame)(nil)

type mockNodeFrame struct {
	depNode          depNode
	parentCallerCtxs runtm.CallerCtxs // unused here
}

func (m *mockNodeFrame) GetDepNode() depNode                   { return m.depNode }
func (m *mockNodeFrame) GetParentCallerCtxs() runtm.CallerCtxs { return m.parentCallerCtxs }
func (m *mockNodeFrame) CreateLinkedNodeFrame(next *linkedNodeFrame, nextCallerCtxs runtm.CallerCtxs) *linkedNodeFrame {
	return nil
}

func TestNodeFramesStack_BasicOps(t *testing.T) {
	stack := newNodeFramesStack()
	require.Equal(t, 0, stack.Len())

	frame1 := &mockNodeFrame{depNode: nil}
	frame2 := &mockNodeFrame{depNode: nil}
	stack.Push(frame1)
	stack.Push(frame2)
	require.Equal(t, 2, stack.Len())
	require.Equal(t, frame2, stack.Peek())

	top := stack.Pop()
	require.Equal(t, frame2, top)
	require.Equal(t, 1, stack.Len())
	require.Equal(t, frame1, stack.Peek())

	stack.Pop()
	require.Equal(t, 0, stack.Len())
}

func TestNodeFramesStack_IterateFromTop(t *testing.T) {
	stack := newNodeFramesStack()
	frames := []nodeFrame{
		&mockNodeFrame{depNode: nil}, &mockNodeFrame{depNode: nil}, &mockNodeFrame{depNode: nil},
	}
	for _, f := range frames {
		stack.Push(f)
	}
	var iterated []nodeFrame
	stack.IterateFromTop()(func(f nodeFrame) bool {
		iterated = append(iterated, f)
		return true
	})
	require.Len(t, iterated, len(frames))
	for i := range frames {
		require.Equal(t, frames[len(frames)-1-i], iterated[i])
	}
}

func TestNodeFramesStack_IterateFromTop_BreakEarly(t *testing.T) {
	stack := newNodeFramesStack()
	frames := []nodeFrame{&mockNodeFrame{}, &mockNodeFrame{}, &mockNodeFrame{}}
	for _, f := range frames {
		stack.Push(f)
	}
	var called int
	stack.IterateFromTop()(func(f nodeFrame) bool {
		called++
		return false
	})
	require.Equal(t, 1, called)
}
