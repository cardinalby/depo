package depo

import (
	"fmt"

	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/tests"
)

const pendingDepNodeIsWaitingForOwnFrames = -1

// pendingDepNodeRecord retains temporary auxiliary info for nodes that are not registered yet
type pendingDepNodeRecord struct {
	// nodes this node depends on
	dependsOn map[depNode]struct{}

	// nodes directly depending on this pending node with callerCtxs describing calls inside
	// dependent provider frame that leads to dependency call
	dependents map[depNode]runtm.CallerCtxs

	// lcHookBuilders collected during the node's providing phase. Considered valid only
	// if the node is successfully registered
	lcHookBuilders []*lifecycleHookBuilder

	// lcHooks are build from lcHookBuilders after the node is registered and passed to the node
	// as a part of the registration result
	lcHooks []*lifecycleHook

	// -1 if the node is awaiting own frames, otherwise it contains the number of node's dependencies
	// that are not registered yet
	waitsForDepsCount int

	hasOwnOrTransitiveRunnables bool
}

func (rec *pendingDepNodeRecord) getDependsOnSlice() []depNode {
	if len(rec.dependsOn) == 0 {
		return nil
	}
	dependsOnSlice := make([]depNode, 0, len(rec.dependsOn))
	for dep := range rec.dependsOn {
		dependsOnSlice = append(dependsOnSlice, dep)
	}
	return dependsOnSlice
}

type pendingNodes map[depNode]pendingDepNodeRecord

func (pn pendingNodes) Add(
	dependent depNode,
	dependency depNode,
	dependencyCallerCtxs runtm.CallerCtxs,
) {
	switch dependency.GetRegState() {
	case nodeRegStateNone:
		dependencyRec, hasDependencyRec := pn[dependency]
		if !hasDependencyRec {
			dependencyRec = pendingDepNodeRecord{
				dependents:        make(map[depNode]runtm.CallerCtxs),
				waitsForDepsCount: pendingDepNodeIsWaitingForOwnFrames,
				// don't initialize waitsForDeps for now
			}
			pn[dependency] = dependencyRec
		}

		if dependent != nil {
			dependentRec, hasDependentRec := pn[dependent]
			if hasDependentRec {
				dependencyRec.dependents[dependent] = dependencyCallerCtxs
				if dependentRec.dependsOn == nil {
					dependentRec.dependsOn = make(map[depNode]struct{})
				}
				dependentRec.dependsOn[dependency] = struct{}{}
				pn[dependent] = dependentRec

			} else if
			// if the dependent is not in pending nodes, it means it has been already removed
			// because of error or late init
			//goland:noinspection GoBoolExpressions
			tests.IsTestingBuild && dependent.GetRegState() != nodeRegStateWithNoLcHooks {
				panic(fmt.Sprintf(
					"dependent %s is not in pending nodes and is not nodeRegStateFailed",
					dependent.GetDepInfo().String(),
				))
			}
		}

	case nodeRegStateWithLcHooks:
		if tests.IsTestingBuild {
			if _, hasDependencyRec := pn[dependency]; hasDependencyRec {
				panic(fmt.Sprintf(
					"dependency %s has nodeRegStateWithLcHooks, and still in pending nodes",
					dependency.GetDepInfo().String(),
				))
			}
		}
		if dependent != nil {
			dependentRec := pn.mustGetRec(dependent)
			if dependentRec.dependsOn == nil {
				dependentRec.dependsOn = make(map[depNode]struct{})
			}
			dependentRec.dependsOn[dependency] = struct{}{}
			dependentRec.hasOwnOrTransitiveRunnables = true
			pn[dependent] = dependentRec
		} else if tests.IsTestingBuild {
			panic(fmt.Sprintf(
				"dependency %s has nodeRegStateWithLcHooks, added with no dependent",
				dependency.GetDepInfo().String(),
			))
		}

	default:
		// dependencies with nodeRegStateWithNoLcHooks should not come here, taking the shortcut in the registry
		// goland:noinspection GoBoolExpressions
		if tests.IsTestingBuild {
			panic(fmt.Sprintf(
				"unexpected nodeRegState %d for dependency %s",
				dependency.GetRegState(),
				dependency.GetDepInfo().String(),
			))
		}
	}
}

func (pn pendingNodes) AddLifecycleHookBuilder(node depNode, lcHookBuilder *lifecycleHookBuilder) {
	rec, hasRec := pn[node]
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && !hasRec {
		panic(fmt.Sprintf("depNode %s has no rec", node.GetDepInfo().String()))
	}
	rec.lcHookBuilders = append(rec.lcHookBuilders, lcHookBuilder)
	// don't set rec.hasOwnOrTransitiveRunnables since some hooks can appear empty
	pn[node] = rec
}

// OnLastNodeFramePoppedWithNoErr is called when the last frame of a node is popped from the stack
// and there are no queued late init frames
func (pn pendingNodes) OnLastNodeFramePoppedWithNoErr(node depNode) {
	rec, hasRec := pn[node]
	if !hasRec {
		// has been already removed by OnDepFrameErr
		//goland:noinspection GoBoolExpressions
		if tests.IsTestingBuild && node.GetRegState() == nodeRegStateNone {
			panic(fmt.Sprintf(
				"OnLastNodeFramePoppedWithNoErr(%s) called for nodeRegStateNone node that is not in pending nodes",
				node.GetDepInfo().Id.String(),
			))
		}
		return
	}
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && node.GetRegState() != nodeRegStateNone {
		panic(fmt.Sprintf(
			"OnLastNodeFramePoppedWithNoErr(%s) called for a node that is not in nodeRegStateNone",
			node.GetDepInfo().Id.String(),
		))
	}

	rec.waitsForDepsCount = 0
	for dependency := range rec.dependsOn {
		rec.waitsForDepsCount += pn.countDepsTransitivelyWaitingForOwnFrames(
			dependency,
			map[depNode]struct{}{node: {}}, // don't visit itself
		)
	}

	for _, lcHookBuilder := range rec.lcHookBuilders {
		if hook, hasRoles := lcHookBuilder.getHook(); hasRoles {
			rec.lcHooks = append(rec.lcHooks, hook)
		}
	}
	rec.lcHookBuilders = nil
	if len(rec.lcHooks) > 0 {
		rec.hasOwnOrTransitiveRunnables = true
	}

	// should assign anyway to check runnables in remove()
	pn[node] = rec

	if rec.waitsForDepsCount == 0 {
		pn.remove(pendingNodeRemoveArgs{
			node: node,
			// empty regErr, framesToFailedLateInit
		})
	}
}

// OnDepFrameErr is called when a dependency frame has an error. If `reportErrToNode` is false,
// the error is not reported to the node itself, but only to the dependents
func (pn pendingNodes) OnDepFrameErr(node depNode, err error, reportErrToNode bool) {
	pn.remove(pendingNodeRemoveArgs{
		node:            node,
		regErr:          err,
		isLateInitErr:   false,
		reportErrToNode: reportErrToNode,
	})
	// If the node is already removed at the topmost frame, remove() will skip it.
	// Even though the error can differ from the initially reported one,
	// we report only the single err for a node (can be improved in the future).
}

// OnDepLateInitFrameErr is called when lateInit function returns an error
func (pn pendingNodes) OnDepLateInitFrameErr(err errDepRegFailed) {
	pn.remove(pendingNodeRemoveArgs{
		node:            err.failedFrame.depNode,
		regErr:          err,
		isLateInitErr:   true,
		reportErrToNode: true,
	})
}

// countDepsTransitivelyWaitingForOwnFrames count the number of node's dependencies that are waiting for their
// own frames or are transitively waiting for nodes that are waiting for their own frames
func (pn pendingNodes) countDepsTransitivelyWaitingForOwnFrames(
	node depNode,
	visited map[depNode]struct{},
) (waitingCount int) {
	if _, isVisited := visited[node]; isVisited {
		return 0
	}
	visited[node] = struct{}{}

	rec, hasRec := pn[node]
	if !hasRec {
		// not in pending nodes, has already received reg result
		return 0
	}
	if rec.waitsForDepsCount == pendingDepNodeIsWaitingForOwnFrames {
		return 1
	}
	if rec.waitsForDepsCount == 0 {
		return 0
	}
	// check if the dependency of the node can be considered as directly/transitively waiting for own frames
	// (discard it if it depends on the `visited` nodes in cycle)
	for dependency := range rec.dependsOn {
		waitingCount += pn.countDepsTransitivelyWaitingForOwnFrames(dependency, visited)
		if waitingCount >= rec.waitsForDepsCount {
			// no need to count more, we already have the expected number of waiting dependencies. If the counter
			// is consistent with dependencies states, there should be no more waiting dependencies
			//goland:noinspection GoBoolExpressions
			if !tests.IsTestingBuild {
				break
			} else if waitingCount > rec.waitsForDepsCount {
				panic(fmt.Sprintf(
					"waitingCount %d > rec.waitsForDepsCount %d for node %s",
					waitingCount,
					rec.waitsForDepsCount,
					node.GetDepInfo().String(),
				))
			}
		}
	}
	return waitingCount
}

func (pn pendingNodes) hasDirectOrTransitiveRunnables(
	node depNode,
	visited map[depNode]struct{},
) bool {
	if _, isVisited := visited[node]; isVisited {
		return false
	}
	visited[node] = struct{}{}

	rec, hasRec := pn[node]
	if !hasRec {
		return node.GetRegState() == nodeRegStateWithLcHooks
	}
	// even though rec.waitsForDepsCount can be 0 in case of cycle dependencies, if we get into this
	// method we are about to remove the node whose frames finished successfully.
	// It means it doesn't wait for any real dependencies (not for itself in a cycle) so we consider
	// all runnables reachable from this node as valid (will be eventually successfully registered)
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && rec.waitsForDepsCount == pendingDepNodeIsWaitingForOwnFrames {
		panic(fmt.Sprintf(
			"node %s in hasDirectOrTransitiveRunnables is waiting for own frames",
			node.GetDepInfo().String(),
		))
	}

	if rec.hasOwnOrTransitiveRunnables || len(rec.lcHooks) > 0 {
		return true
	}
	for dependency := range rec.dependsOn {
		if pn.hasDirectOrTransitiveRunnables(dependency, visited) {
			return true
		}
	}
	return false
}

type pendingNodeRemoveArgs struct {
	node            depNode
	regErr          error
	isLateInitErr   bool
	reportErrToNode bool
}

func (pn pendingNodes) remove(args pendingNodeRemoveArgs) {
	rec, hasRec := pn[args.node]
	if !hasRec {
		return // stop cyclic removal
	}

	for ownDependency := range rec.dependsOn {
		dependencyRec, hasDependencyRec := pn[ownDependency]
		if hasDependencyRec {
			delete(dependencyRec.dependents, args.node)
		}
	}

	var dependentsRemoveArgs []pendingNodeRemoveArgs

	for dependent, depCallerCtxs := range rec.dependents {
		dependentRec, hasDependentRec := pn[dependent]
		if !hasDependentRec {
			// if the dependent is not in pending nodes, it means it has been already recursively removed it
			continue
		}
		// do not depend on the failed node
		if args.regErr != nil {
			delete(dependentRec.dependsOn, args.node)
		}

		if dependentRec.waitsForDepsCount == pendingDepNodeIsWaitingForOwnFrames {
			// - dependent can receive new dependencies to wait for
			// - it can't be a late init error affecting dependent
			// - in case of error we should wait for the dependent to receive its own frames
			//   wrapping the error
			continue
		}

		//goland:noinspection GoBoolExpressions
		if tests.IsTestingBuild && dependentRec.waitsForDepsCount == 0 {
			panic(fmt.Sprintf(
				"dependent %s waitsForDepsCount %d == 0 while removing dependency %s",
				dependent.GetDepInfo().String(),
				dependentRec.waitsForDepsCount,
				args.node.GetDepInfo().String(),
			))
		}
		// some dependent can wait for this node to receive runnables not having own runnables
		dependentRec.waitsForDepsCount--
		// update rec in pending nodes only if dependent is not going to be removed

		if args.isLateInitErr || dependentRec.waitsForDepsCount == 0 {
			// forcibly and recursively remove dependents in case of:
			// - late init error: dependent can't deliberately ignore the error in the dependency (and not use it)
			//   if it happened after the dependent successfully obtained the dependency. This way dependent
			//   now has a failed dependency and doesn't know about it
			// - successful registration of all dependencies
			dependentRemoveArgs := pendingNodeRemoveArgs{
				node:            dependent,
				isLateInitErr:   args.isLateInitErr,
				reportErrToNode: true, // always report error to waiting dependents
			}
			if args.isLateInitErr {
				dependentRemoveArgs.regErr = pn.tailorDependentLateInitErr(dependent, depCallerCtxs, args.regErr)
			} else {
				dependentRemoveArgs.regErr = args.regErr
			}
			dependentsRemoveArgs = append(dependentsRemoveArgs, dependentRemoveArgs)
		} else {
			pn[dependent] = dependentRec
		}
	}
	if args.regErr != nil {
		rec.lcHookBuilders = nil
		rec.dependsOn = nil
		rec.hasOwnOrTransitiveRunnables = false
	} else if !rec.hasOwnOrTransitiveRunnables {
		rec.hasOwnOrTransitiveRunnables = pn.hasDirectOrTransitiveRunnables(
			args.node, map[depNode]struct{}{},
		)
	}
	delete(pn, args.node)

	errToReport := args.regErr
	if !args.reportErrToNode {
		errToReport = nil
	}
	args.node.SetRegResult(
		errToReport,
		rec.getDependsOnSlice(),
		rec.lcHooks,
		rec.hasOwnOrTransitiveRunnables,
	)

	for _, rmArgs := range dependentsRemoveArgs {
		pn.remove(rmArgs)
	}
}

func (pn pendingNodes) tailorDependentLateInitErr(
	dependent depNode,
	depCallerCtxs runtm.CallerCtxs,
	err error,
) error {
	// errDepRegFailed can't be wrapped here
	//goland:noinspection GoTypeAssertionOnErrors
	depRegFailedErr, ok := err.(errDepRegFailed)
	if !ok {
		if tests.IsTestingBuild {
			panic(fmt.Sprintf(
				"tailorDependentLateInitErr called with non errDepRegFailed error %T: %s",
				err, err.Error(),
			))
		}
		return err
	}
	// reconstruct the dependent's provider frame
	depRegFailedErr.failedFrame = &linkedNodeFrame{
		depNode:        dependent,
		next:           depRegFailedErr.failedFrame,
		nextCallerCtxs: depCallerCtxs,
	}
	return depRegFailedErr
}

func (pn pendingNodes) mustGetRec(node depNode) pendingDepNodeRecord {
	rec, hasRec := pn[node]
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && !hasRec {
		panic(fmt.Sprintf("depNode %s has no rec", node.GetDepInfo().String()))
	}
	return rec
}
