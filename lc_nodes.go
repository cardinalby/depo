package depo

import (
	"github.com/cardinalby/depo/internal/tests"
)

type lcNodes []*lcNode

type lcNodesGraph struct {
	roots      lcNodes
	leaves     lcNodes
	totalCount int
}

// newLcNodesGraph transforms the sub-graph of `rootNode` dependencies to lcNodes graph where each lcNode
// corresponds a lifecycleHook added to a depNode and returns the root lcNodes.
// returns errCyclicDependency if topological sort is not possible
func newLcNodesGraph(rootNode depNode) (graph lcNodesGraph, err error) {
	// use depth-first search marking nodes with temporary and permanent marks
	tempMarks := make(map[depNode]struct{})
	permMarks := make(map[depNode]struct{})

	var sorted lcNodes
	// maps depNode to its transitive runnable dependencies (for non-runnable nodes)
	transitiveLcDeps := make(map[depNode]map[*lcNode]struct{})
	// contains lcNodes created for depNode with allowed circular dependency at second visit.
	// lcNodes have empty dependsOn and should be filled at the end of first visit
	preCreatedLcNodes := make(map[depNode]lcNodes)

	createLcNodesForDepNode := func(node depNode, dependsOn lcNodes) lcNodes {
		nodeTransitiveRunnableDeps := make(map[*lcNode]struct{})
		lcHooks := node.GetLifecycleHooks()
		nodeOwnLcNodes := make(lcNodes, 0, len(lcHooks))
		for _, lcHook := range lcHooks {
			lcNode := newLcNode(node, lcHook, dependsOn)
			nodeOwnLcNodes = append(nodeOwnLcNodes, lcNode)
			nodeTransitiveRunnableDeps[lcNode] = struct{}{}
		}
		transitiveLcDeps[node] = nodeTransitiveRunnableDeps
		return nodeOwnLcNodes
	}

	var visitingStack visitingNodesStack
	var visit func(depNode) error
	visit = func(node depNode) error {
		visitingStack.Push(node)
		defer func() {
			visitingStack.Pop()
		}()

		if _, ok := permMarks[node]; ok {
			// already permanently marked, no need to visit again
			return nil
		}
		if _, ok := tempMarks[node]; ok {
			// already temporarily marked, this means we have a cycle. But it doesn't immediately mean that
			// runnables are in cycle that would prevent them from being run in the proper order.
			//
			// 1. If `depNode` has runnables and the cycle goes only through nodes that have no runnables
			//    (initialized with LateInits) to the initial depNode with runnables
			// 2. If `depNode` has no runnables, but the cycle goes through nodes that have runnables
			//
			// In both cases, we still can run these runnables not taking cycling nodes into account.
			// Check `visitingStack` to detect such cases

			runnableNodesInCycleCount := 0
			if len(node.GetLifecycleHooks()) > 0 {
				runnableNodesInCycleCount++
			}
			// iterate intermediate nodes between two `depNode` entries:
			// skip the last depNode in visitingStack since it quals `depNode`,
			// iterate till the next entry of `depNode`
			for i := len(visitingStack) - 2; i >= 0 && visitingStack[i] != node; i-- {
				if len(visitingStack[i].GetLifecycleHooks()) > 0 {
					runnableNodesInCycleCount++
					if runnableNodesInCycleCount > 1 {
						break
					}
				}
			}
			if runnableNodesInCycleCount > 1 {
				return newRunnablesCyclicDependencyError(visitingStack)
			}
			// no runnables in the cycle, don't go inside the same depNode again
			if len(node.GetLifecycleHooks()) > 0 {
				// it's a second `visit` call for `depNode`.
				// pre-create lcNodes so that dependents can see them in transitiveLcDeps and `node` can
				// pick them up at the end of first `visit`. Don't fill dependsOn yet, it will be done
				// at the end of first `visit` call
				preCreatedLcNodes[node] = createLcNodesForDepNode(node, nil)
			}
			return nil
		}

		tempMarks[node] = struct{}{}
		defer delete(tempMarks, node)

		// Visit all dependencies first
		for _, depNode := range node.GetDependsOn() {
			if err := visit(depNode); err != nil {
				return err
			}
		}

		permMarks[node] = struct{}{}

		// dependencies should have been already visited at this point, collect their
		// direct/transitive lcNodes. All lcNodes of this depNode share the same dependsOnSlice
		var dependsOnSlice lcNodes
		dependsOnSet := make(map[*lcNode]struct{})
		for _, depNode := range node.GetDependsOn() {
			// Collect transitive runnable dependencies from depNode dependencies
			for lcNode := range transitiveLcDeps[depNode] {
				if _, isAdded := dependsOnSet[lcNode]; !isAdded && lcNode.depNode != node {
					// ignore dependency on itself and already added dependencies
					dependsOnSlice = append(dependsOnSlice, lcNode)
					dependsOnSet[lcNode] = struct{}{}
				}
			}
		}
		dependsOnSet = nil

		if len(node.GetLifecycleHooks()) > 0 {
			lcNodes := preCreatedLcNodes[node]
			delete(preCreatedLcNodes, node)

			if len(lcNodes) > 0 {
				// there is an allowed circular dependency, we have already visited this node and
				// pre-created lcNodes for it, but they don't have dependsOn set yet.
				for _, lcNode := range lcNodes {
					lcNode.dependsOn = dependsOnSlice
				}
			} else {
				lcNodes = createLcNodesForDepNode(node, dependsOnSlice)
			}
			for _, lcNode := range lcNodes {
				sorted = append(sorted, lcNode)
			}
		} else if len(dependsOnSlice) > 0 {
			// has no own lcNodes, fill transitiveLcDeps
			nodeTransitiveRunnableDeps := make(map[*lcNode]struct{})
			for _, lcNodeDependency := range dependsOnSlice {
				nodeTransitiveRunnableDeps[lcNodeDependency] = struct{}{}
			}
			transitiveLcDeps[node] = nodeTransitiveRunnableDeps
		}

		return nil
	}

	if err := visit(rootNode); err != nil {
		return graph, err
	}

	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && len(preCreatedLcNodes) > 0 {
		panic("preCreatedLcNodes is not empty after visiting root")
	}

	// collect dependents for each lcNode. At this point there are no cycles in the graph
	dependentsOf := make(map[*lcNode]map[*lcNode]struct{}, len(sorted))

	lcNodesSetToSlice := func(set map[*lcNode]struct{}) lcNodes {
		slice := make(lcNodes, 0, len(set))
		for node := range set {
			slice = append(slice, node)
		}
		return slice
	}

	for _, node := range sorted {
		for _, dependency := range node.dependsOn {
			dependents, ok := dependentsOf[dependency]
			if !ok {
				dependents = make(map[*lcNode]struct{})
			}
			dependents[node] = struct{}{}
			dependentsOf[dependency] = dependents
		}
	}
	for _, node := range sorted {
		if len(node.dependsOn) == 0 {
			graph.leaves = append(graph.leaves, node)
		}
		if _, ok := dependentsOf[node]; ok {
			node.dependents = lcNodesSetToSlice(dependentsOf[node])
		} else {
			graph.roots = append(graph.roots, node)
		}
	}

	graph.totalCount = len(sorted)

	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild && graph.totalCount != 0 && (len(graph.roots) == 0 || len(graph.leaves) == 0) {
		panic("lcNodes graph has no roots or leaves but there are lcNodes")
	}

	return graph, nil
}
