package depo

import "context"

// lcNode is a lifecycle depNode that represents a runnable with its dependencies
type lcNode struct {
	lcNodeOwnInfo
	dependents lcNodes
	dependsOn  lcNodes
	// improvement: move to lifecycleHook to share its state between different Runners allowing simultaneous runs
	runState lcNodeRunState
}

func newLcNode(
	depNode depNode,
	lcHook *lifecycleHook,
	dependsOn lcNodes,
) *lcNode {
	node := &lcNode{
		lcNodeOwnInfo: lcNodeOwnInfo{
			depNode: depNode,
			lcHook:  lcHook,
		},
		dependsOn: dependsOn,
	}
	return node
}

func (n *lcNode) DependsOnHooks() []LifecycleHookNode {
	if len(n.dependsOn) == 0 {
		return nil
	}
	res := make([]LifecycleHookNode, 0, len(n.dependsOn))
	for _, dependency := range n.dependsOn {
		res = append(res, dependency)
	}
	return res
}

type lcNodePhaseDoneState uint8

const (
	lcNodePhaseDoneStateNone lcNodePhaseDoneState = iota
	lcNodePhaseDoneStateSkipped
	lcNodePhaseDoneStateCompleted
)

type lcNodeRunState struct {
	isStarting         bool
	isStartDone        lcNodePhaseDoneState
	cancelStartCtx     context.CancelCauseFunc
	isWaiting          bool
	isWaitDone         lcNodePhaseDoneState
	isClosing          bool
	isCloseDone        lcNodePhaseDoneState
	closedDependencies int
	doneDependents     int
	readyDeps          int
}

func (s *lcNodeRunState) IsDone() bool {
	return s.isWaitDone > 0 && s.isCloseDone > 0
}
