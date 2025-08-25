package depo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cardinalby/depo/internal/tests"
)

var errAlreadyRunning = errors.New("is already running")

type nodeErrResult struct {
	node *lcNode
	err  error
}

type runnerSession struct {
	graph         lcNodesGraph
	config        runnerCfg
	onReady       func()
	shutdownCause error
	shutdownErr   error
	stats         *lcNodesStats
	startResults  chan nodeErrResult
	closeResults  chan *lcNode
	waitResults   chan nodeErrResult
}

func newRunnerSession(
	graph lcNodesGraph,
	onReady func(),
	config runnerCfg,
) runnerSession {
	if onReady == nil {
		onReady = func() {}
	}
	return runnerSession{
		graph:        graph,
		config:       config,
		onReady:      onReady,
		stats:        newLcNodesStats(graph.totalCount),
		startResults: make(chan nodeErrResult),
		closeResults: make(chan *lcNode),
		waitResults:  make(chan nodeErrResult),
	}
}

func (rs *runnerSession) run(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if len(rs.graph.leaves) == 0 {
		rs.onReady()
		return nil
	}

	for _, node := range rs.graph.leaves {
		if done := rs.tryStartNode(node); done {
			return rs.shutdownErr
		}
	}

	return rs.loop(ctx)
}

func (rs *runnerSession) loop(ctx context.Context) error {
	for {
		select {
		case startRes := <-rs.startResults:
			if done := rs.handleNodeStartResult(startRes, lcNodePhaseDoneStateCompleted); done {
				return rs.shutdownErr
			}

		case waitRes := <-rs.waitResults:
			if done := rs.handleNodeWaited(waitRes, lcNodePhaseDoneStateCompleted); done {
				return rs.shutdownErr
			}

		case closedNode := <-rs.closeResults:
			if done := rs.handleNodeClosed(closedNode, lcNodePhaseDoneStateCompleted); done {
				return rs.shutdownErr
			}

		case <-ctx.Done():
			if done := rs.tryShutDown(ctx.Err(), context.Cause(ctx)); done {
				return rs.shutdownErr
			}
			// once closed, ctx.Done will always make select choose this case, start another loop
			// without <-ctx.Done case
			return rs.loopAfterCtxDone()
		}
	}
}

func (rs *runnerSession) loopAfterCtxDone() error {
	for {
		select {
		case startRes := <-rs.startResults:
			if done := rs.handleNodeStartResult(startRes, lcNodePhaseDoneStateCompleted); done {
				return rs.shutdownErr
			}

		case waitRes := <-rs.waitResults:
			if done := rs.handleNodeWaited(waitRes, lcNodePhaseDoneStateCompleted); done {
				return rs.shutdownErr
			}

		case closedNode := <-rs.closeResults:
			if done := rs.handleNodeClosed(closedNode, lcNodePhaseDoneStateCompleted); done {
				return rs.shutdownErr
			}
		}
	}
}

func (rs *runnerSession) handleNodeClosed(node *lcNode, doneState lcNodePhaseDoneState) (done bool) {
	node.runState.isClosing = false
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && node.runState.isCloseDone > 0 {
		panic(fmt.Sprintf("node %v is already closed", node.Tag()))
	}
	node.runState.isCloseDone = doneState
	rs.stats.remainingCloses--
	rs.stats.onNodeDependentsHaveClosedDependency(node)

	if node.runState.IsDone() {
		rs.stats.onNodeDependenciesHaveDoneDependent(node)
		return rs.tryCloseNodeDependencies(node)
	}
	// if we are here it means wait is not done
	return false
}

func (rs *runnerSession) tryCloseNodeDependencies(node *lcNode) (done bool) {
	for _, dependency := range node.dependsOn {
		if done := rs.tryCloseNode(dependency, rs.shutdownCause); done {
			return true
		}
	}
	return rs.stats.isAllDone()
}

func (rs *runnerSession) tryCloseNode(node *lcNode, cause error) (done bool) {
	switch {
	case node.runState.isStarting || node.runState.isClosing:
		// don't Close:
		// - starting nodes. The only reason to stop starting node is initiated shutdown, in this case
		//   they are stopped by startingCtx cancellation and should eventually report start result
		// - closing nodes. They are already closing and will report close result
		return false

	case node.runState.isCloseDone > 0:
		// the node is already closed by itself but its dependencies are not closed yet
		if node.runState.closedDependencies < len(node.dependsOn) {
			return rs.tryCloseNodeDependencies(node)
		}
		return rs.stats.isAllDone()

	case node.runState.isStartDone == lcNodePhaseDoneStateNone:
		// the node had no chance to be started/waited because of shutdown
		node.runState.isStartDone = lcNodePhaseDoneStateSkipped
		// skip wait phase
		node.runState.isWaitDone = lcNodePhaseDoneStateSkipped
		rs.stats.remainingWaits--

		if node.lcHook.starter != nil {
			// don't close Starter if it hasn't been started yet
			return rs.handleNodeClosed(node, lcNodePhaseDoneStateSkipped)
		}

	case node.runState.isWaiting:
		//goland:noinspection GoBoolExpressions
		if node.runState.doneDependents < len(node.dependents) {
			return false
		} else if tests.IsTestingBuild && node.runState.doneDependents > len(node.dependents) {
			panic(fmt.Sprintf("node %v has more done dependents (%d) than it has (%d) dependents",
				node.Tag(), node.runState.doneDependents, len(node.dependents),
			))
		}

	default:
	}

	rs.config.runnerListeners.OnClose(node.lcNodeOwnInfo, cause)

	if !node.lcHook.hasCloser() || node.lcHook.waiter != nil && node.runState.isWaitDone > 0 {
		return rs.handleNodeClosed(node, lcNodePhaseDoneStateSkipped)
	}

	if node.lcHook.isTrustedAsyncCloser() {
		// don't start a goroutine if Close is guaranteed to be non-blocking
		node.lcHook.close(cause)
		return rs.handleNodeClosed(node, lcNodePhaseDoneStateCompleted)
	}

	node.runState.isClosing = true
	go func() {
		node.lcHook.close(cause)
		rs.closeResults <- node
	}()
	return false
}

func (rs *runnerSession) tryShutDown(err error, cause error) (done bool) {
	if rs.shutdownErr != nil {
		return rs.stats.isAllDone()
	}
	rs.config.runnerListeners.OnShutdown(cause)
	rs.shutdownErr = err
	rs.shutdownCause = cause
	rs.interruptStarts(cause)

	for _, node := range rs.graph.roots {
		if done := rs.tryCloseNode(node, cause); done {
			return true
		}
	}
	return rs.stats.isAllDone()
}

func (rs *runnerSession) interruptStarts(cause error) {
	var interruptNodeStart func(node *lcNode)
	interruptNodeStart = func(node *lcNode) {
		if node.runState.cancelStartCtx != nil {
			node.runState.cancelStartCtx(cause)
		}
		for _, dependency := range node.dependsOn {
			interruptNodeStart(dependency)
		}
	}
	for _, node := range rs.graph.roots {
		interruptNodeStart(node)
	}
}

func (rs *runnerSession) handleNodeWaited(waitRes nodeErrResult, doneState lcNodePhaseDoneState) (done bool) {
	rs.stats.remainingWaits--
	waitRes.node.runState.isWaiting = false
	waitRes.node.runState.isWaitDone = doneState

	if doneState == lcNodePhaseDoneStateCompleted {
		if tests.IsTestingBuild && waitRes.node.lcHook.waiter == nil {
			panic(fmt.Sprintf("handleNodeWaited(%v) with no waiter", waitRes.node.Tag()))
		}
		if waitRes.err == nil && rs.isNilRunResultAsError(waitRes.node) {
			waitRes.err = ErrUnexpectedRunNilResult
		}
		if !waitRes.node.runState.isClosing && waitRes.node.runState.isCloseDone == lcNodePhaseDoneStateNone {
			waitRes.node.runState.isCloseDone = lcNodePhaseDoneStateSkipped
			rs.stats.remainingCloses--
		}
	}
	if waitRes.node.runState.IsDone() {
		rs.config.runnerListeners.OnDone(waitRes.node.lcNodeOwnInfo, waitRes.err)
		rs.stats.onNodeDependenciesHaveDoneDependent(waitRes.node)
	}
	if rs.shutdownErr == nil && waitRes.err != nil {
		waitRes.err = newErrLifecycleHookFailed(waitRes.node, failedLifecyclePhaseWait, waitRes.err)
		if done := rs.tryShutDown(waitRes.err, waitRes.err); done {
			return true
		}
	}
	if rs.shutdownErr != nil {
		return rs.tryCloseNode(waitRes.node, rs.shutdownCause)
	}

	return rs.stats.isAllDone()
}

func (rs *runnerSession) tryWaitForNode(node *lcNode) (done bool) {
	if node.runState.isWaiting {
		return false
	}
	if node.runState.isWaitDone > 0 {
		return rs.stats.isAllDone()
	}
	if node.lcHook.waiter == nil {
		return rs.handleNodeWaited(
			nodeErrResult{node: node, err: nil},
			lcNodePhaseDoneStateSkipped,
		)
	}

	node.runState.isWaiting = true
	go func() {
		err := node.lcHook.waiter.wait()
		rs.waitResults <- nodeErrResult{node: node, err: err}
	}()
	return false
}

func (rs *runnerSession) handleNodeStartResult(startRes nodeErrResult, doneState lcNodePhaseDoneState) (done bool) {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && startRes.node.runState.readyDeps != len(startRes.node.dependsOn) {
		panic(fmt.Sprintf("node %v started with %d ready deps but has %d dependencies",
			startRes.node.Tag(), startRes.node.runState.readyDeps, len(startRes.node.dependsOn)))
	}
	startRes.node.runState.isStarting = false
	startRes.node.runState.isStartDone = doneState
	if startRes.node.runState.cancelStartCtx != nil {
		// start ctx can be timeout context that needs to be cancelled to stop the internal timer
		startRes.node.runState.cancelStartCtx(startRes.err)
	}

	if startRes.err != nil {
		return rs.handleNodeStartError(startRes)
	}

	if done := rs.handleNodeIsReady(startRes.node); done {
		return true
	}

	if done := rs.tryWaitForNode(startRes.node); done {
		return true
	}

	if rs.shutdownErr != nil {
		// even though start was successful, and we started waiting for the node, shutdown is in progress and
		// may have skipped this node since it was starting. Try to close if possible now.
		// Don't report readiness and don't start new nodes
		return rs.tryCloseNode(startRes.node, rs.shutdownCause)
	}

	return rs.stats.isAllDone()
}

func (rs *runnerSession) handleNodeStartError(startRes nodeErrResult) (done bool) {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && startRes.node.lcHook.starter == nil {
		panic(fmt.Sprintf("handleNodeStartResult(%v) with err with no starterMock", startRes.node.Tag()))
	}
	// Increment done counters for its dependencies to unblock their shutdown
	rs.stats.onNodeDependenciesHaveDoneDependent(startRes.node)
	startRes.err = newErrLifecycleHookFailed(startRes.node, failedLifecyclePhaseStart, startRes.err)
	// the node has no chance to be waited if start failed. If we are here, it means the node has Starter
	// and doesn't need to be waited if it failed to start
	startRes.node.runState.isCloseDone = lcNodePhaseDoneStateSkipped
	rs.stats.remainingCloses--
	startRes.node.runState.isWaitDone = lcNodePhaseDoneStateSkipped
	rs.stats.remainingWaits--
	rs.config.runnerListeners.OnDone(startRes.node.lcNodeOwnInfo, startRes.err)
	if done := rs.tryShutDown(startRes.err, startRes.err); done {
		return true
	}
	return rs.tryCloseNodeDependencies(startRes.node)
}

func (rs *runnerSession) handleNodeIsReady(node *lcNode) (done bool) {
	rs.config.runnerListeners.OnReady(node.lcNodeOwnInfo)
	rs.stats.remainingReadySignals--

	// consider ready / start new nodes only if shutdown is not in progress
	if rs.shutdownErr == nil {
		if rs.stats.remainingReadySignals == 0 {
			rs.onReady()
			// don't need to start new nodes if all nodes are ready
		} else {
			for _, dependent := range node.dependents {
				dependent.runState.readyDeps++
				if done := rs.tryStartNode(dependent); done {
					return true
				}
			}
		}
	}
	return rs.stats.isAllDone()
}

func (rs *runnerSession) tryStartNode(node *lcNode) (done bool) {
	if node.runState.isStartDone > 0 {
		if tests.IsTestingBuild {
			panic(fmt.Sprintf("node %v is already started", node.Tag()))
		}
		return rs.stats.isAllDone()
	}
	if node.runState.isStarting {
		return false
	}
	if node.runState.readyDeps < len(node.dependsOn) {
		return false
	}
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && node.runState.readyDeps > len(node.dependsOn) {
		panic(fmt.Sprintf("node %v has more ready dependencies (%d) than it has (%d) dependencies",
			node.Tag(), node.runState.readyDeps, len(node.dependsOn)))
	}

	rs.config.runnerListeners.OnStart(node.lcNodeOwnInfo)
	if node.lcHook.starter == nil {
		return rs.handleNodeStartResult(nodeErrResult{node: node, err: nil}, lcNodePhaseDoneStateSkipped)
	}

	if _, isAsync := node.lcHook.starter.(trustedAsyncStarter); isAsync {
		// don't start a goroutine if Start is guaranteed to be non-blocking
		err := node.lcHook.starter.Start(nil)
		return rs.handleNodeStartResult(nodeErrResult{node: node, err: err}, lcNodePhaseDoneStateCompleted)
	}

	node.runState.isStarting = true
	var startCtx context.Context
	startCtx, node.runState.cancelStartCtx = rs.getStartCtx(node)
	go func() {
		err := node.lcHook.starter.Start(startCtx)
		rs.startResults <- nodeErrResult{node: node, err: err}
	}()
	return false
}

func (rs *runnerSession) isNilRunResultAsError(node *lcNode) bool {
	return rs.config.nilResultAsError || node.lcHook.waiterCfg.nilResultAsError
}

func (rs *runnerSession) getStartCtx(node *lcNode) (context.Context, context.CancelCauseFunc) {
	var startTimeout time.Duration
	if node.lcHook.starterCfg.startTimeout != 0 {
		startTimeout = node.lcHook.starterCfg.startTimeout
	} else {
		startTimeout = rs.config.startTimeout
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	if startTimeout <= 0 {
		return ctx, cancel
	}

	// no need to explicitly cancel timeoutCtx if it's cancelled by parent via cancel(), it would be no-op
	// (internal timer is already stopped, and it has been removed from the parent context)
	//goland:noinspection GoVetLostCancel
	timeoutCtx, _ := context.WithTimeout(ctx, startTimeout)
	return timeoutCtx, cancel
}

type lcNodesStats struct {
	remainingWaits        int
	remainingCloses       int
	remainingReadySignals int
}

func newLcNodesStats(nodesCount int) *lcNodesStats {
	return &lcNodesStats{
		remainingReadySignals: nodesCount,
		remainingWaits:        nodesCount,
		remainingCloses:       nodesCount,
	}
}

func (s *lcNodesStats) isAllDone() bool {
	if tests.IsTestingBuild {
		if s.remainingWaits < 0 {
			panic(fmt.Sprintf("remainingWaits is negative: %d", s.remainingWaits))
		}
		if s.remainingCloses < 0 {
			panic(fmt.Sprintf("remainingCloses is negative: %d", s.remainingCloses))
		}
	}

	return s.remainingWaits+s.remainingCloses == 0
}

func (s *lcNodesStats) onNodeDependenciesHaveDoneDependent(node *lcNode) {
	for _, dependency := range node.dependsOn {
		dependency.runState.doneDependents++
	}
}

func (s *lcNodesStats) onNodeDependentsHaveClosedDependency(node *lcNode) {
	for _, dependent := range node.dependents {
		dependent.runState.closedDependencies++
	}
}
