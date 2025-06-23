package depo

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type runnableOrderRequirements []runnableOrderRequirement

type runnableOrderRequirement struct {
	tag       any
	dependsOn []any
}

// Inverse dependencies direction
func (r runnableOrderRequirements) Inverse() runnableOrderRequirements {
	dependents := make(map[any][]any, len(r))
	for _, req := range r {
		for _, dep := range req.dependsOn {
			dependents[dep] = append(dependents[dep], req.tag)
		}
	}
	inverseReqs := make(runnableOrderRequirements, 0, len(r))
	for _, req := range r {
		inverseReqs = append(inverseReqs, runnableOrderRequirement{
			tag:       req.tag,
			dependsOn: dependents[req.tag],
		})
	}
	return inverseReqs
}

// require `tag` item to be root, relink dependents
func (r runnableOrderRequirements) RemoveDependenciesOnAndRelink(tag any) runnableOrderRequirements {
	var tagDependsOn []any
	for _, req := range r {
		if req.tag == tag {
			tagDependsOn = req.dependsOn
			break
		}
	}
	res := make(runnableOrderRequirements, 0, len(r))
	for _, req := range r {
		it := runnableOrderRequirement{
			tag:       req.tag,
			dependsOn: make([]any, 0, len(req.dependsOn)),
		}
		for _, dep := range req.dependsOn {
			if dep != tag {
				it.dependsOn = append(it.dependsOn, dep)
			} else {
				it.dependsOn = append(it.dependsOn, tagDependsOn...)
			}
		}
		res = append(res, it)
	}
	return res
}

func (r runnableOrderRequirements) Remove(tag any) runnableOrderRequirements {
	res := make(runnableOrderRequirements, 0, len(r))
	for _, req := range r {
		it := runnableOrderRequirement{
			tag:       req.tag,
			dependsOn: make([]any, 0, len(req.dependsOn)),
		}
		if req.tag == tag {
			continue
		}
		for _, dep := range req.dependsOn {
			if dep != tag {
				it.dependsOn = append(it.dependsOn, dep)
			}
		}
		res = append(res, it)
	}
	return res
}

func (r runnableOrderRequirements) Filter(
	filterFn func(req runnableOrderRequirement) bool,
) (res runnableOrderRequirements) {
	for _, req := range r {
		if filterFn(req) {
			res = append(res, req)
		}
	}
	return res
}

func (r runnableOrderRequirements) CheckLcNodesGraph(
	t *testing.T,
	graph lcNodesGraph,
) {
	visited := make(map[*lcNode]struct{})
	nodes := make(map[any]*lcNode)
	var visit func(node *lcNode)
	visit = func(node *lcNode) {
		if _, ok := visited[node]; ok {
			return
		}
		visited[node] = struct{}{}
		nodes[node.Tag()] = node
		for _, dep := range node.dependsOn {
			visit(dep)
		}
	}
	for _, node := range graph.roots {
		visit(node)
		require.Nil(t, node.dependents)
	}
	for _, node := range graph.leafs {
		require.Nil(t, node.dependsOn)
	}
	getDepsTags := func(node *lcNode) []any {
		tags := make([]any, 0, len(node.dependsOn))
		for _, depNode := range node.dependsOn {
			tags = append(tags, depNode.Tag())
		}
		return tags
	}
	require.Equal(t, len(r), len(nodes))
	for _, req := range r {
		lcNode, ok := nodes[req.tag]
		if !ok {
			t.Fatalf("runnable %v not found in lcNodesGraph", req.tag)
		}
		depsTags := getDepsTags(lcNode)
		require.ElementsMatch(t,
			req.dependsOn,
			depsTags,
			"tag %v dependencies %v do not match req %v", req.tag, depsTags, req.dependsOn)
	}
}

func (r runnableOrderRequirements) findReq(t *testing.T, tag any) runnableOrderRequirement {
	for _, req := range r {
		if req.tag == tag {
			return req
		}
	}
	t.Fatalf("%v not found in requirements", tag)
	return runnableOrderRequirement{}
}

func (r runnableOrderRequirements) checkOrderMap(
	t *testing.T,
	orderMap map[any]int,
) {
	for _, req := range r {
		dependentOrder, exists := orderMap[req.tag]
		if !exists {
			continue
		}
		for _, dep := range req.dependsOn {
			depOrder, exists := orderMap[dep]
			if !exists {
				continue
			}
			if depOrder >= dependentOrder {
				require.Fail(
					t,
					"invalid order",
					"%v should be before %v in order map %v",
					dep, req.tag, orderMap,
				)
			}
		}
	}
}

func (r runnableOrderRequirements) checkEventTimings(
	t *testing.T,
	events []runnerListenerMockEvent,
	getRunnableDuration func(tag any) time.Duration,
) {
	var firstEventStart time.Time
	for _, event := range events {
		if firstEventStart.IsZero() || event.moment.Before(firstEventStart) {
			firstEventStart = event.moment
		}
	}
	expectedTimingsMap := r.getTagExpectedTimings(t, firstEventStart, getRunnableDuration)
	for _, event := range events {
		expTiming := expectedTimingsMap[event.info.Tag()]
		if event.moment.Before(expTiming.min) || (!expTiming.max.IsZero() && event.moment.After(expTiming.max)) {
			require.Fail(t, "timings mismatch",
				"event %s for %v at %s is out of expected timings [%s, %s]",
				event.eventType, event.info.Tag(),
				event.moment, expTiming.min, expTiming.max)
		}
	}
}

type expectedTimings struct {
	min time.Time
	max time.Time
}

// assumes no cyclic dependencies
func (r runnableOrderRequirements) getTagExpectedTimings(
	t *testing.T,
	baseTime time.Time,
	getRunnableDuration func(tag any) time.Duration,
) map[any]expectedTimings {
	res := make(map[any]expectedTimings, len(r))

	minLeafD := time.Duration(math.MaxInt64)
	for _, req := range r {
		if len(req.dependsOn) == 0 {
			d := getRunnableDuration(req.tag)
			if d < minLeafD {
				minLeafD = d
			}
		}
	}

	type depChainDurationStats struct {
		ownD     time.Duration
		maxDepsD time.Duration
	}
	longestDependenciesChainDurations := make(map[any]depChainDurationStats, len(r))
	var visitDependencies func(req runnableOrderRequirement)
	visitDependencies = func(req runnableOrderRequirement) {
		if _, ok := longestDependenciesChainDurations[req.tag]; ok {
			return
		}
		var stats depChainDurationStats
		for _, dep := range req.dependsOn {
			depReq := r.findReq(t, dep)
			visitDependencies(depReq)
			depStats := longestDependenciesChainDurations[depReq.tag]
			depTotalD := depStats.ownD + depStats.maxDepsD
			if depTotalD > stats.maxDepsD {
				stats.maxDepsD = depTotalD
			}
		}
		stats.ownD = getRunnableDuration(req.tag)
		longestDependenciesChainDurations[req.tag] = stats
	}
	for _, req := range r {
		visitDependencies(req)
	}
	minDependentsOfDurations := make(map[any]time.Duration, len(r))
	for _, req := range r {
		ownLongestD := longestDependenciesChainDurations[req.tag].maxDepsD
		for _, dep := range req.dependsOn {
			depMinD, ok := minDependentsOfDurations[dep]
			if !ok || ownLongestD < depMinD {
				minDependentsOfDurations[dep] = ownLongestD
			}
		}
	}
	for _, req := range r {
		exp := expectedTimings{}
		exp.min = baseTime.Add(longestDependenciesChainDurations[req.tag].maxDepsD)
		maxD, hasMaxD := minDependentsOfDurations[req.tag]
		if hasMaxD {
			exp.max = baseTime.Add(maxD)
		}
		res[req.tag] = exp
	}
	return res
}
