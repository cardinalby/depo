package depo

import (
	"fmt"
	"slices"
	"testing"

	"github.com/cardinalby/depo/internal/dep"
	"github.com/cardinalby/depo/internal/tests"
	"github.com/stretchr/testify/require"
)

type testNameProviderI interface {
	getOwnName() string
	getDepNames(visited []testNameProviderI) []string
}

type testDep struct {
	ownName       string
	nameProviders []testNameProviderI
}

func (d *testDep) getOwnName() string {
	return d.ownName
}

func (d *testDep) getDepNames(visited []testNameProviderI) []string {
	names := []string{d.ownName}
	for _, provider := range d.nameProviders {
		if !slices.Contains(visited, provider) {
			names = append(names, provider.getDepNames(append(visited, d))...)
		}
	}
	return names
}

var _ testNameProviderI = (*testDep)(nil)

func TestAddLateInit(t *testing.T) {
	t.Run("cyclic dependencies", func(t *testing.T) {

		t.Run("depends on itself", func(t *testing.T) {
			t.Run("simple", func(t *testing.T) {
				t.Cleanup(testsCleanup)
				// 1 -> 1
				var def1 func() *testDep
				def1 = Provide(func() *testDep {
					def1()
					return &testDep{
						ownName: "1",
					}
				})
				tests.RequirePanicsWithErrorIs(t, func() {
					def1()
				}, errCyclicDependency)
				require.Empty(t, globalRegistry.pendingNodes)
			})

			t.Run("via late init", func(t *testing.T) {
				t.Cleanup(testsCleanup)
				var def1 func() *testDep
				def1 = Provide(func() *testDep {
					require.Equal(t, uint64(1), UseComponentID())
					d := &testDep{
						ownName: "1",
					}
					UseLateInit(func() {
						require.Equal(t, uint64(1), UseComponentID())
						def1()
					})
					return d
				})
				require.Equal(t, "1", def1().getOwnName())
				require.Empty(t, globalRegistry.pendingNodes)
			})
		})

		t.Run("simple direct cycle", func(t *testing.T) {
			t.Cleanup(testsCleanup)
			// 1 -> 2 <-> 3
			initOrders := [][]int{
				{1, 2, 3},
				{1, 3, 2},
				{2, 3, 1},
				{2, 1, 3},
				{3, 1, 2},
				{3, 2, 1},
				{1, 2},
				{1, 3},
				{1},
			}
			isResolvingCycle := func(lateInitIndexes []int) bool {
				return slices.Contains(lateInitIndexes, 2) ||
					slices.Contains(lateInitIndexes, 3)
			}
			lateInitIndexesSet := [][]int{
				{2},
				{3},
				{2, 3},
				{1, 2, 3},
				{1},
				{},
			}
			hasRunnableIndexesSet := [][]int{
				{},
				{1},
				{2},
				{3},
				{1, 2},
				{1, 3},
				{2, 3},
				{1, 2, 3},
			}
			for _, initOrder := range initOrders {
				for _, lateInitIndexes := range lateInitIndexesSet {
					for _, hasRunnableIndexes := range hasRunnableIndexesSet {
						t.Run(fmt.Sprintf(
							"init order: %v, lateInits: %v, hasRunnable: %v",
							initOrder, lateInitIndexes, hasRunnableIndexes,
						), func(t *testing.T) {
							t.Cleanup(testsCleanup)

							var runnable1, runnable2, runnable3 *readinessRunnableMock
							if slices.Contains(hasRunnableIndexes, 1) {
								runnable1 = newReadinessRunnableMock(1)
							}
							if slices.Contains(hasRunnableIndexes, 2) {
								runnable2 = newReadinessRunnableMock(2)
							}
							if slices.Contains(hasRunnableIndexes, 3) {
								runnable3 = newReadinessRunnableMock(3)
							}

							var def1, def2, def3 func() *testDep
							if slices.Contains(lateInitIndexes, 1) {
								def1 = Provide(func() *testDep {
									d := &testDep{
										ownName: "1",
									}
									if runnable1 != nil {
										UseLifecycle().AddReadinessRunnable(runnable1).Tag(1)
									}
									UseLateInit(func() {
										d.nameProviders = []testNameProviderI{def2()}
									})
									return d
								})
							} else {
								def1 = Provide(func() *testDep {
									if runnable1 != nil {
										UseLifecycle().AddReadinessRunnable(runnable1).Tag(1)
									}
									return &testDep{
										ownName:       "1",
										nameProviders: []testNameProviderI{def2()},
									}
								})
							}
							if slices.Contains(lateInitIndexes, 2) {
								def2 = Provide(func() *testDep {
									d := &testDep{
										ownName: "2",
									}
									if runnable2 != nil {
										UseLifecycle().AddReadinessRunnable(runnable2).Tag(2)
									}
									UseLateInit(func() {
										d.nameProviders = []testNameProviderI{def3()}
									})
									return d
								})
							} else {
								def2 = Provide(func() *testDep {
									if runnable2 != nil {
										UseLifecycle().AddReadinessRunnable(runnable2).Tag(2)
									}
									return &testDep{
										ownName:       "2",
										nameProviders: []testNameProviderI{def3()},
									}
								})
							}
							if slices.Contains(lateInitIndexes, 3) {
								def3 = Provide(func() *testDep {
									d := &testDep{
										ownName: "3",
									}
									if runnable3 != nil {
										UseLifecycle().AddReadinessRunnable(runnable3).Tag(3)
									}
									UseLateInit(func() {
										d.nameProviders = []testNameProviderI{def2()}
									})
									return d
								})
							} else {
								def3 = Provide(func() *testDep {
									if runnable3 != nil {
										UseLifecycle().AddReadinessRunnable(runnable3).Tag(3)
									}
									return &testDep{
										ownName:       "3",
										nameProviders: []testNameProviderI{def2()},
									}
								})
							}

							for _, callIdx := range initOrder {
								switch callIdx {
								case 1:
									if isResolvingCycle(lateInitIndexes) {
										require.Equal(t, "1", def1().getOwnName())
										require.Equal(t, []string{"1", "2", "3"}, def1().getDepNames(nil))
										if runnable1 != nil {
											require.Equal(t, createdNodes[1].GetLifecycleHooks()[0].tag, 1)
										}
										if runnable1 != nil || runnable2 != nil || runnable3 != nil {
											require.Equal(t, nodeRegStateWithLcHooks, createdNodes[1].GetRegState())
										}
									} else {
										tests.RequirePanicsWithErrorIs(t, func() {
											def1()
										}, ErrCyclicDependency)
									}
									require.Empty(t, globalRegistry.pendingNodes)
								case 2:
									if isResolvingCycle(lateInitIndexes) {
										require.Equal(t, "2", def2().getOwnName())
										require.Equal(t, []string{"2", "3"}, def2().getDepNames(nil))
										if runnable2 != nil {
											require.Equal(t, createdNodes[2].GetLifecycleHooks()[0].tag, 2)
										}
										if runnable2 != nil || runnable3 != nil {
											require.Equal(t, nodeRegStateWithLcHooks, createdNodes[2].GetRegState())
										}
									} else {
										tests.RequirePanicsWithErrorIs(t, func() {
											def2()
										}, ErrCyclicDependency)
									}
									require.Empty(t, globalRegistry.pendingNodes)
								case 3:
									if isResolvingCycle(lateInitIndexes) {
										require.Equal(t, "3", def3().getOwnName())
										require.Equal(t, []string{"3", "2"}, def3().getDepNames(nil))
										if runnable3 != nil {
											require.Equal(t, createdNodes[3].GetLifecycleHooks()[0].tag, 3)
										}
										if runnable3 != nil || runnable2 != nil {
											require.Equal(t, nodeRegStateWithLcHooks, createdNodes[3].GetRegState())
										}
									} else {
										tests.RequirePanicsWithErrorIs(t, func() {
											def3()
										}, ErrCyclicDependency)
									}
									require.Empty(t, globalRegistry.pendingNodes)
								default:
									t.Fatalf("unexpected call index: %d", callIdx)
								}
							}
							rr, err := NewRunnerE(func() error {
								for _, callIdx := range initOrder {
									switch callIdx {
									case 1:
										def1()
									case 2:
										def2()
									case 3:
										def3()
									default:
										t.Fatalf("unexpected call index: %d", callIdx)
									}
								}
								return nil
							})
							if !isResolvingCycle(lateInitIndexes) {
								require.ErrorIs(t, err, errCyclicDependency)
								return
							}
							if slices.Contains(hasRunnableIndexes, 2) && slices.Contains(hasRunnableIndexes, 3) {
								require.ErrorIs(t, err, errCyclicDependency)
							} else {
								// should successfully create a runner
								var lcNodesRequirements runnableOrderRequirements
								if runnable1 != nil {
									r1Requirement := runnableOrderRequirement{
										tag: 1,
									}
									if runnable2 != nil {
										r1Requirement.dependsOn = append(r1Requirement.dependsOn, 2)
									} else if runnable3 != nil {
										r1Requirement.dependsOn = append(r1Requirement.dependsOn, 3)
									}
									lcNodesRequirements = append(lcNodesRequirements, r1Requirement)
								}
								if runnable2 != nil {
									r2Requirement := runnableOrderRequirement{
										tag: 2,
									}
									if runnable3 != nil {
										r2Requirement.dependsOn = append(r2Requirement.dependsOn, 3)
									}
									lcNodesRequirements = append(lcNodesRequirements, r2Requirement)
								}
								if runnable3 != nil {
									r3Requirement := runnableOrderRequirement{
										tag: 3,
									}
									if runnable2 != nil {
										r3Requirement.dependsOn = append(r3Requirement.dependsOn, 2)
									}
									lcNodesRequirements = append(lcNodesRequirements, r3Requirement)
								}
								rrImpl, ok := rr.(*runner)
								require.True(t, ok)
								lcNodesRequirements.CheckLcNodesGraph(t, rrImpl.graph)
							}
						})
					}
				}
			}
		})

		t.Run("indirect cycle", func(t *testing.T) {
			t.Cleanup(testsCleanup)
			// 0 -> 1 -> 2 -> 0
			initOrders := [][]int{
				{0, 1, 2},
				{0, 2, 1},
				{1, 2, 0},
				{1, 0, 2},
				{2, 0, 1},
				{2, 1, 0},
				{0},
				{1},
				{2},
			}
			isResolvingCycle := func(lateInitIndexes []int) bool {
				return slices.Contains(lateInitIndexes, 1) ||
					slices.Contains(lateInitIndexes, 2) ||
					slices.Contains(lateInitIndexes, 0)
			}
			lateInitIndexesSet := [][]int{
				{0},
				{1},
				{2},
				{0, 1},
				{0, 2},
				{1, 2},
				{0, 1, 2},
				{},
			}
			hasRunnableIndexesSet := [][]int{
				{},
				{0},
				{1},
				{2},
				{0, 1},
				{0, 2},
				{1, 2},
				{0, 1, 2},
			}
			for _, initOrder := range initOrders {
				for _, lateInitIndexes := range lateInitIndexesSet {
					for _, runnableIndexes := range hasRunnableIndexesSet {
						var runnable0, runnable1, runnable2 *readinessRunnableMock
						if slices.Contains(runnableIndexes, 0) {
							runnable0 = newReadinessRunnableMock(0)
						}
						if slices.Contains(runnableIndexes, 1) {
							runnable1 = newReadinessRunnableMock(1)
						}
						if slices.Contains(runnableIndexes, 2) {
							runnable2 = newReadinessRunnableMock(2)
						}

						t.Run(fmt.Sprintf(
							"init order: %v, lateInits: %v, runnables: %v",
							initOrder, lateInitIndexes, runnableIndexes,
						), func(t *testing.T) {
							t.Cleanup(testsCleanup)

							var def0, def1, def2 func() *testDep
							if slices.Contains(lateInitIndexes, 0) {
								def0 = Provide(func() *testDep {
									d := &testDep{
										ownName: "0",
									}
									if runnable0 != nil {
										UseLifecycle().AddReadinessRunnable(runnable0).Tag(0)
									}
									UseLateInit(func() {
										d.nameProviders = []testNameProviderI{def1()}
									})
									return d
								})
							} else {
								def0 = Provide(func() *testDep {
									if runnable0 != nil {
										UseLifecycle().AddReadinessRunnable(runnable0).Tag(0)
									}
									return &testDep{
										ownName:       "0",
										nameProviders: []testNameProviderI{def1()},
									}
								})
							}
							if slices.Contains(lateInitIndexes, 1) {
								def1 = Provide(func() *testDep {
									d := &testDep{
										ownName: "1",
									}
									if runnable1 != nil {
										UseLifecycle().AddReadinessRunnable(runnable1).Tag(1)
									}
									UseLateInit(func() {
										d.nameProviders = []testNameProviderI{def2()}
									})
									return d
								})
							} else {
								def1 = Provide(func() *testDep {
									if runnable1 != nil {
										UseLifecycle().AddReadinessRunnable(runnable1).Tag(1)
									}
									return &testDep{
										ownName:       "1",
										nameProviders: []testNameProviderI{def2()},
									}
								})
							}
							if slices.Contains(lateInitIndexes, 2) {
								def2 = Provide(func() *testDep {
									d := &testDep{
										ownName: "2",
									}
									if runnable2 != nil {
										UseLifecycle().AddReadinessRunnable(runnable2).Tag(2)
									}
									UseLateInit(func() {
										d.nameProviders = []testNameProviderI{def0()}
									})
									return d
								})
							} else {
								def2 = Provide(func() *testDep {
									if runnable2 != nil {
										UseLifecycle().AddReadinessRunnable(runnable2).Tag(2)
									}
									return &testDep{
										ownName:       "2",
										nameProviders: []testNameProviderI{def0()},
									}
								})
							}

							for _, callIdx := range initOrder {
								switch callIdx {
								case 0:
									if isResolvingCycle(lateInitIndexes) {
										require.Equal(t, "0", def0().getOwnName())
										require.Equal(t, []string{"0", "1", "2"}, def0().getDepNames(nil))
										if runnable0 != nil {
											require.Equal(t, createdNodes[1].GetLifecycleHooks()[0].tag, 0)
										}
										if len(runnableIndexes) > 0 {
											require.Equal(t, nodeRegStateWithLcHooks, createdNodes[1].GetRegState())
										}
									} else {
										tests.RequirePanicsWithErrorIs(t, func() {
											def0()
										}, ErrCyclicDependency)
									}
									require.Empty(t, globalRegistry.pendingNodes)
								case 1:
									if isResolvingCycle(lateInitIndexes) {
										require.Equal(t, "1", def1().getOwnName())
										require.Equal(t, []string{"1", "2", "0"}, def1().getDepNames(nil))
										if runnable1 != nil {
											require.Equal(t, createdNodes[2].GetLifecycleHooks()[0].tag, 1)
										}
										if len(runnableIndexes) > 0 {
											require.Equal(t, nodeRegStateWithLcHooks, createdNodes[2].GetRegState())
										}
									} else {
										tests.RequirePanicsWithErrorIs(t, func() {
											def1()
										}, errCyclicDependency)
									}
									require.Empty(t, globalRegistry.pendingNodes)
								case 2:
									if isResolvingCycle(lateInitIndexes) {
										require.Equal(t, "2", def2().getOwnName())
										require.Equal(t, []string{"2", "0", "1"}, def2().getDepNames(nil))
										if runnable2 != nil {
											require.Equal(t, createdNodes[3].GetLifecycleHooks()[0].tag, 2)
										}
										if len(runnableIndexes) > 0 {
											require.Equal(t, nodeRegStateWithLcHooks, createdNodes[3].GetRegState())
										}
									} else {
										tests.RequirePanicsWithErrorIs(t, func() {
											def2()
										}, ErrCyclicDependency)
									}
									require.Empty(t, globalRegistry.pendingNodes)
								}
							}

							rr, err := NewRunnerE(func() error {
								for _, callIdx := range initOrder {
									switch callIdx {
									case 0:
										def0()
									case 1:
										def1()
									case 2:
										def2()
									default:
										t.Fatalf("unexpected call index: %d", callIdx)
									}
								}
								return nil
							})
							if !isResolvingCycle(lateInitIndexes) || len(runnableIndexes) > 1 {
								require.ErrorIs(t, err, errCyclicDependency)
								return
							}
							rrImpl, ok := rr.(*runner)
							require.True(t, ok)

							if len(runnableIndexes) == 0 {
								require.Empty(t, rrImpl.graph)
								return
							}

							// should successfully create a runner with a single runnable
							var lcNodesRequirements runnableOrderRequirements
							if runnable0 != nil {
								lcNodesRequirements = append(lcNodesRequirements, runnableOrderRequirement{
									tag: 0,
								})
							}
							if runnable1 != nil {
								lcNodesRequirements = append(lcNodesRequirements, runnableOrderRequirement{
									tag: 1,
								})
							}
							if runnable2 != nil {
								lcNodesRequirements = append(lcNodesRequirements, runnableOrderRequirement{
									tag: 2,
								})
							}
							lcNodesRequirements.CheckLcNodesGraph(t, rrImpl.graph)
						})
					}
				}
			}
		})
	})

	t.Run("nested late inits", func(t *testing.T) {
		var calls []int

		type testDep struct {
			own  int
			deps []*testDep
		}

		existingRunnablesSet := [][]int{
			{},
			{1},
			{1, 2},
			{1, 2, 3},
			{1, 2, 3, 11},
			{1, 2, 3, 11, 12},
			{1, 2, 3, 11, 12, 13},
			{1, 2, 3, 11, 12, 13, 21},
			{1, 2, 3, 11, 12, 13, 21, 22},
			{3},
			{2, 3},
			{2, 3, 13},
			{2},
			{2, 22},
			{2, 22, 13},
		}
		for _, existingRunnableTags := range existingRunnablesSet {
			t.Run(fmt.Sprintf("existing runnables: %v", existingRunnableTags), func(t *testing.T) {
				t.Cleanup(testsCleanup)
				runnables := make(map[int]ReadinessRunnable, len(existingRunnableTags))
				for _, tag := range existingRunnableTags {
					runnables[tag] = newReadinessRunnableMock(tag)
				}
				onProviderCalled := func(tag int) {
					calls = append(calls, tag)
					if runnable, ok := runnables[tag]; ok {
						UseLifecycle().AddReadinessRunnable(runnable).Tag(tag)
					}
				}

				var def1, def2, def3 func() *testDep

				// 3 <------ 1
				// 3 -> 2 -> 1
				// 1: late inits:
				//   1.1 -> 2
				//      1.2
				//      1.3 -> 3
				// 2: late inits:
				//   2.1
				//   2.2 -> 3
				createDefs := func() {
					def1 = Provide(func() *testDep {
						onProviderCalled(1)
						d := &testDep{
							own: 1,
						}
						UseLateInit(func() {
							onProviderCalled(11)

							UseLateInit(func() {
								onProviderCalled(12)
							})

							d.deps = append(d.deps, def2())

							UseLateInit(func() {
								onProviderCalled(13)
								d.deps = append(d.deps, def3())
							})
						})
						return d
					})

					def2 = Provide(func() *testDep {
						onProviderCalled(2)

						UseLateInit(func() {
							onProviderCalled(21)
						})
						d := &testDep{
							own:  2,
							deps: []*testDep{def1()},
						}
						UseLateInit(func() {
							onProviderCalled(22)
							d.deps = append(d.deps, def3())
						})
						return d
					})

					def3 = Provide(func() *testDep {
						onProviderCalled(3)
						return &testDep{
							own:  3,
							deps: []*testDep{def1(), def2()},
						}
					})
				}

				validateDeps := func(t *testing.T, dep1, dep2, dep3 *testDep) {
					t.Helper()
					require.Len(t, dep1.deps, 2)
					require.Equal(t, dep1.deps[0].own, 2)
					require.Equal(t, dep1.deps[1].own, 3)
					require.Len(t, dep2.deps, 2)
					require.Equal(t, dep2.deps[0].own, 1)
					require.Equal(t, dep2.deps[1].own, 3)
					require.Len(t, dep3.deps, 2)
					require.Equal(t, dep3.deps[0].own, 1)
					require.Equal(t, dep3.deps[1].own, 2)

					require.Len(t, createdNodes, 3)
					require.Len(t, globalRegistry.pendingNodes, 0)

					expectedTagsForNodes := map[dep.Id][]int{}
					for _, existingTag := range existingRunnableTags {
						var depId dep.Id
						switch existingTag {
						case 1, 11, 12, 13:
							depId = 1
						case 2, 21, 22:
							depId = 2
						case 3:
							depId = 3
						}
						expectedTagsForNodes[depId] = append(expectedTagsForNodes[depId], existingTag)
					}

					// check runnables in created nodes
					for _, node := range createdNodes {
						nodeLcHooks := node.GetLifecycleHooks()
						expectedNodeTags := expectedTagsForNodes[node.GetDepInfo().Id]
						require.Len(t, nodeLcHooks, len(expectedNodeTags),
							"node: %s", node.GetDepInfo().Id)
						for _, expectedTag := range expectedNodeTags {
							hasLcHook := slices.ContainsFunc(nodeLcHooks, func(rt *lifecycleHook) bool {
								return rt.tag == expectedTag
							})
							require.True(t, hasLcHook, "node: %s, expected lcRoles: %v",
								node.GetDepInfo().Id, expectedTag,
							)
						}
					}

					rr, err := NewRunnerE(func() error {
						def1()
						def2()
						def3()
						return nil
					})
					if len(expectedTagsForNodes) > 1 {
						require.ErrorIs(t, err, errCyclicDependency)
					} else {
						require.NoError(t, err)
						rrImpl, ok := rr.(*runner)
						require.True(t, ok)
						require.Equal(t, len(existingRunnableTags), rrImpl.graph.totalCount)
					}
				}

				t.Run("1 2 3", func(t *testing.T) {
					t.Cleanup(testsCleanup)
					createDefs()
					dep1 := def1()
					require.Equal(t,
						[]int{1, 11, 2, 12, 21, 22, 3, 13},
						calls,
					)
					calls = nil
					dep2 := def2()
					require.Empty(t, calls)
					dep3 := def3()
					require.Empty(t, calls)
					validateDeps(t, dep1, dep2, dep3)
				})

				t.Run("3 2 1", func(t *testing.T) {
					t.Cleanup(testsCleanup)
					createDefs()
					dep3 := def3()
					require.Equal(t,
						[]int{3, 1, 2, 11, 21, 22, 12, 13},
						calls,
					)
					calls = nil
					dep2 := def2()
					require.Empty(t, calls)
					dep1 := def1()
					require.Empty(t, calls)
					validateDeps(t, dep1, dep2, dep3)
				})

				t.Run("2 1 3", func(t *testing.T) {
					t.Cleanup(testsCleanup)
					createDefs()
					dep2 := def2()
					require.Equal(t,
						[]int{2, 1, 21, 11, 22, 3, 12, 13},
						calls,
					)
					calls = nil
					require.Empty(t, calls)
					dep1 := def1()
					require.Empty(t, calls)
					dep3 := def3()
					validateDeps(t, dep1, dep2, dep3)
				})
			})
		}
	})

	t.Run("nil late init fn", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		defs := []func() int{
			Provide(func() int {
				UseLateInit(nil)
				t.Fatal("should not reach here")
				return 1
			}),
			Provide(func() int {
				UseLateInitE(nil)
				t.Fatal("should not reach here")
				return 1
			}),
		}
		errMsgRequirement := new(errMsgRequirement).
			Raw(newErrNilLateInit().Error()).
			FileLines(1)
		for _, def := range defs {
			tests.RequirePanicsWithErrorIs(t, func() {
				def()
			}, errNilValue, func(err error) {
				checkErrorMsgLines(t, err, errMsgRequirement)
			})
		}

		defsErr := []func() (int, error){
			ProvideE(func() (int, error) {
				UseLateInit(nil)
				t.Fatal("should not reach here")
				return 1, nil
			}),
			ProvideE(func() (int, error) {
				UseLateInitE(nil)
				t.Fatal("should not reach here")
				return 1, nil
			}),
		}
		for _, defErr := range defsErr {
			_, err := defErr()
			checkErrorMsgLines(t, err, errMsgRequirement)
		}
	})
}

func TestUseLateInit_NotInProvideContext(t *testing.T) {
	t.Cleanup(testsCleanup)

	tests.RequirePanicsWithErrorIs(t, func() {
		UseLateInit(func() {})
	}, errNotInProviderFn)

	tests.RequirePanicsWithErrorIs(t, func() {
		UseLateInitE(func() error { return nil })
	}, errNotInProviderFn)

	def1 := Provide(func() int {
		done := make(chan struct{})
		go func() {
			defer close(done)
			tests.RequirePanicsWithErrorIs(t, func() {
				UseLateInit(func() {})
			}, errNotInProviderFn)
		}()
		<-done
		return 1
	})
	def1()
}
