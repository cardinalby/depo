package depo

import (
	"strings"

	"github.com/cardinalby/depo/internal/strs"
	"github.com/cardinalby/depo/internal/tests"
)

// improvement: keep callerCtxs for frames to show the full trace like in errDepRegFailed. We don't do this
// because that information exists only in pendingNodes in the process of deps resolving.
// NewRunnerE may be never called (the library is used only for Provides) and we don't want to keep
// the whole pendingNodes in memory
type errRunnablesCyclicDependencyStruct struct {
	nodesStack visitingNodesStack
}

func newRunnablesCyclicDependencyError(nodesStack visitingNodesStack) error {
	if len(nodesStack) == 0 {
		if tests.IsTestingBuild {
			panic("newRunnablesCyclicDependencyError: nodesStack is empty")
		}
		return errCyclicDependency
	}
	return errRunnablesCyclicDependencyStruct{
		nodesStack: nodesStack.CloneSkipLowest(),
	}
}

func (e errRunnablesCyclicDependencyStruct) Error() string {
	var lines strs.Lines
	markedDepId := e.nodesStack.Peek().GetDepInfo().Id
	lines.Push(errCyclicDependency.Error() + " between components with UseLifecycle:")
	lastIdx := len(e.nodesStack) - 1
	padding := strings.Repeat(" ", lnFrameNamePrefixLen)
	for i := lastIdx; i >= 0; i-- {
		prefix := padding
		node := e.nodesStack[i]
		if node.GetDepInfo().Id == markedDepId {
			prefix = lnFrameNamePrefixArrow
		}
		lines.Push(prefix + node.GetDepInfo().String())
	}
	return lines.Join()
}

func (e errRunnablesCyclicDependencyStruct) Unwrap() error {
	return errCyclicDependency
}
