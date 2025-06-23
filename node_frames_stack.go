package depo

import (
	"github.com/cardinalby/depo/internal/tests"
)

type readonlyNodeFramesStack interface {
	Peek() nodeFrame
	IterateFromTop() func(yield func(frame nodeFrame) bool)
	Len() int
}

type nodeFramesStack struct {
	frames []nodeFrame
}

func newNodeFramesStack() *nodeFramesStack {
	return &nodeFramesStack{}
}

func (s *nodeFramesStack) Peek() nodeFrame {
	return s.frames[len(s.frames)-1]
}

func (s *nodeFramesStack) Len() int {
	return len(s.frames)
}

func (s *nodeFramesStack) IterateFromTop() func(yield func(frame nodeFrame) bool) {
	return func(yield func(frame nodeFrame) bool) {
		for i := len(s.frames) - 1; i >= 0; i-- {
			if !yield(s.frames[i]) {
				return
			}
		}
	}
}

func (s *nodeFramesStack) Push(frame nodeFrame) {
	s.frames = append(s.frames, frame)
}

func (s *nodeFramesStack) Pop() nodeFrame {
	topIdx := len(s.frames) - 1
	frame := s.frames[topIdx]
	s.frames = s.frames[:topIdx]
	return frame
}

func (s *nodeFramesStack) CleanUp() {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && len(s.frames) != 0 {
		panic("nodeFramesStack is not empty")
	}
	s.frames = nil
}
