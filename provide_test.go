package depo

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/cardinalby/depo/internal/dep"
	"github.com/cardinalby/depo/internal/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testsCleanup() {
	globalRegistry = newRegistry()
	createdNodesMu.Lock()
	createdNodes = make(map[dep.Id]depNode)
	createdNodesMu.Unlock()
	lastRunnableEventSeqNo.Store(0)
}

func TestDefs(t *testing.T) {
	t.Run("ProvideE", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		runnable0 := newReadinessRunnableMock(0)
		runnable1 := newReadinessRunnableMock(1)
		def1Err := errors.New("def1 error")

		def0 := ProvideE(func() (*readinessRunnableMock, error) {
			require.Equal(t, uint64(1), UseComponentID())
			UseLifecycle().AddReadinessRunnable(runnable0).Tag(0)
			return runnable0, nil
		})
		def1 := ProvideE(func() (*readinessRunnableMock, error) {
			def0res, def0err := def0()
			require.Equal(t, uint64(2), UseComponentID())
			require.NoError(t, def0err)
			require.Equal(t, runnable0, def0res)
			UseLifecycle().AddReadinessRunnable(runnable1).Tag(1)
			return runnable1, def1Err
		})

		res0, err := def0()
		require.NoError(t, err)
		require.Equal(t, runnable0, res0)

		res1, err := def1()
		require.ErrorIs(t, err, def1Err)
		require.Equal(t, runnable1, res1)
		require.Empty(t, globalRegistry.pendingNodes)

		rr, err := NewRunnerE(func() error {
			_, _ = def0()
			_, _ = def1()
			return nil
		})
		require.NoError(t, err)
		rrImpl, ok := rr.(*runner)
		require.True(t, ok)
		runnableOrderRequirements{
			{
				tag: 0,
			},
		}.CheckLcNodesGraph(t, rrImpl.graph)
		require.Empty(t, globalRegistry.pendingNodes)
	})

	t.Run("Provide in Provide", func(t *testing.T) {
		t.Run("valid case", func(t *testing.T) {
			t.Cleanup(testsCleanup)

			runnable0 := newReadinessRunnableMock(0)
			runnable1 := newReadinessRunnableMock(1)

			def1 := Provide(func() *readinessRunnableMock {
				def0 := Provide(func() *readinessRunnableMock {
					require.Equal(t, uint64(2), UseComponentID())
					UseLifecycle().AddReadinessRunnable(runnable0).Tag(0)
					return runnable0
				})
				require.Equal(t, uint64(1), UseComponentID())
				def0()
				UseLifecycle().AddReadinessRunnable(runnable1).Tag(1)
				return runnable1
			})
			require.Equal(t, runnable1, def1())
			rr, err := NewRunnerE(func() error {
				def1()
				return nil
			})
			require.NoError(t, err)
			rrImpl, ok := rr.(*runner)
			require.True(t, ok)
			runnableOrderRequirements{
				{
					tag: 0,
				},
				{
					tag:       1,
					dependsOn: []any{0},
				},
			}.CheckLcNodesGraph(t, rrImpl.graph)
			require.Empty(t, globalRegistry.pendingNodes)
		})

		t.Run("example mixed graph", func(t *testing.T) {
			// same configuration as in TestDefsGraph_GetTopoSortedLcNodes but created with defs
			runnable0 := newReadinessRunnableMock(0)
			runnable1 := newReadinessRunnableMock(1)
			runnable5 := newReadinessRunnableMock(5)
			runnable6 := newReadinessRunnableMock(6)

			//    ╭─> 5 ─> 4 ─┬─> 2 ──> 0
			// R ─┤           ↓         ↑
			//    ╰─> 6 ────> 3 ────────┴──> 1
			type defsCollection struct {
				def0 func() *readinessRunnableMock
				def1 func() int
				def2 func() int
				def3 func() int
				def4 func() int
				def5 func() (*readinessRunnableMock, error)
				def6 func() *readinessRunnableMock
			}
			createDefs := func() (res defsCollection) {
				res.def0 = Provide(func() *readinessRunnableMock {
					UseLifecycle().AddReadinessRunnable(runnable0).Tag(0)
					return runnable0
				})
				res.def1 = Provide(func() int {
					UseLifecycle().AddReadinessRunnable(runnable1).Tag(1)
					return 1
				})
				res.def2 = Provide(func() int {
					res.def0()
					return 2
				})
				res.def3 = Provide(func() int {
					res.def0()
					res.def1()
					return 3
				})
				res.def4 = Provide(func() int {
					res.def2()
					res.def3()
					return 4
				})
				res.def5 = ProvideE(func() (*readinessRunnableMock, error) {
					res.def4()
					UseLifecycle().AddReadinessRunnable(runnable5).Tag(5)
					return runnable5, nil
				})
				res.def6 = Provide(func() *readinessRunnableMock {
					res.def3()
					UseLifecycle().AddReadinessRunnable(runnable6).Tag(6)
					return runnable6
				})
				return res
			}

			callDefByIndex := func(t *testing.T, defs defsCollection, idx int) {
				switch idx {
				case 0:
					assert.Equal(t, runnable0, defs.def0())
				case 1:
					assert.Equal(t, 1, defs.def1())
				case 2:
					assert.Equal(t, 2, defs.def2())
				case 3:
					assert.Equal(t, 3, defs.def3())
				case 4:
					assert.Equal(t, 4, defs.def4())
				case 5:
					def5Res, err := defs.def5()
					assert.Equal(t, runnable5, def5Res)
					require.NoError(t, err)
				case 6:
					assert.Equal(t, runnable6, defs.def6())
				}
			}
			createRunnerCheckLcNodes := func(t *testing.T, defs defsCollection) {
				require.Empty(t, globalRegistry.pendingNodes)
				rr, err := NewRunnerE(func() error {
					_, _ = defs.def5()
					defs.def6()
					return nil
				})
				require.NoError(t, err)
				rrImpl, ok := rr.(*runner)
				require.True(t, ok)
				runnableOrderRequirements{
					{
						tag: 0,
					},
					{
						tag: 1,
					},
					{
						tag:       5,
						dependsOn: []any{0, 1},
					},
					{
						tag:       6,
						dependsOn: []any{0, 1},
					},
				}.CheckLcNodesGraph(t, rrImpl.graph)
			}

			t.Run("seq calls", func(t *testing.T) {
				callOrders := [][]int{
					{0, 1, 2, 3, 4, 5, 6},
					{6, 5, 4, 3, 2, 1, 0},
					{4, 5, 6, 3, 2, 1, 0},
					{4, 6, 5, 3, 2, 0, 1},
					{},
				}
				for _, callOrder := range callOrders {
					t.Run(fmt.Sprintf("call order: %v", callOrder), func(t *testing.T) {
						t.Cleanup(testsCleanup)
						defs := createDefs()
						for _, callIdx := range callOrder {
							callDefByIndex(t, defs, callIdx)
						}
						createRunnerCheckLcNodes(t, defs)
					})
				}
			})

			t.Run("concurrent calls", func(t *testing.T) {
				for i := 0; i < 50; i++ {
					t.Run(fmt.Sprintf("iteration %d", i), func(t *testing.T) {
						t.Cleanup(testsCleanup)
						defs := createDefs()

						var wg sync.WaitGroup
						for j := 0; j < 100; j++ {
							wg.Add(1)
							go func(defIdx int) {
								defer wg.Done()
								callDefByIndex(t, defs, defIdx)
							}(j % 7)
						}
						wg.Wait()
						createRunnerCheckLcNodes(t, defs)
					})
				}
			})
		})

		t.Run("with multiple custom runnables", func(t *testing.T) {
			t.Cleanup(testsCleanup)

			runnable0 := newReadinessRunnableMock(0)
			runnable2dot1 := newReadinessRunnableMock(21)
			runnable2dot2 := newReadinessRunnableMock(22)

			def0 := Provide(func() *readinessRunnableMock {
				UseLifecycle().AddReadinessRunnable(runnable0).Tag(0)
				return runnable0
			})
			def1 := Provide(func() int {
				def0()
				return 1
			})
			def2 := Provide(func() int {
				def1()
				UseLifecycle().AddReadinessRunnable(runnable2dot1).Tag(21)
				UseLifecycle().AddReadinessRunnable(runnable2dot2).Tag(22)
				return 2
			})

			require.Equal(t, runnable0, def0())
			require.Equal(t, 1, def1())
			require.Equal(t, 2, def2())

			t.Run("simple root lcRoles", func(t *testing.T) {
				t.Cleanup(testsCleanup)
				rr, err := NewRunnerE(func() error {
					def2()
					return nil
				})
				require.NoError(t, err)
				rrImpl, ok := rr.(*runner)
				require.True(t, ok)
				runnableOrderRequirements{
					{
						tag: 0,
					},
					{
						tag:       21,
						dependsOn: []any{0},
					},
					{
						tag:       22,
						dependsOn: []any{0},
					},
				}.CheckLcNodesGraph(t, rrImpl.graph)
				require.Empty(t, globalRegistry.pendingNodes)
			})

			t.Run("root lcRoles with 2 custom runnables", func(t *testing.T) {
				t.Cleanup(testsCleanup)
				runnableX := newReadinessRunnableMock(100)
				runnableY := newReadinessRunnableMock(101)
				rr, err := NewRunnerE(func() error {
					def2()
					UseLifecycle().AddReadinessRunnable(runnableX).Tag(100)
					UseLifecycle().AddReadinessRunnable(runnableY).Tag(101)
					return nil
				})
				require.NoError(t, err)
				rrImpl, ok := rr.(*runner)
				require.True(t, ok)

				runnableOrderRequirements{
					{
						tag: 0,
					},
					{
						tag:       21,
						dependsOn: []any{0},
					},
					{
						tag:       22,
						dependsOn: []any{0},
					},
					{
						tag:       100,
						dependsOn: []any{21, 22},
					},
					{
						tag:       101,
						dependsOn: []any{21, 22},
					},
				}.CheckLcNodesGraph(t, rrImpl.graph)
				require.Empty(t, globalRegistry.pendingNodes)
			})

			t.Run("root lcRoles with 2 custom runnables and same explicit deps", func(t *testing.T) {
				t.Cleanup(testsCleanup)
				runnableX := newReadinessRunnableMock(100)
				runnableY := newReadinessRunnableMock(101)
				runnableZ := newReadinessRunnableMock(102)
				defZ := Provide(func() ReadinessRunnable {
					UseLifecycle().AddReadinessRunnable(runnableZ).Tag(102)
					return runnableZ
				})
				rr, err := NewRunnerE(func() error {
					def2()
					UseLifecycle().AddReadinessRunnable(Provide(func() ReadinessRunnable {
						defZ()
						return runnableX
					})()).Tag(100)
					UseLifecycle().AddReadinessRunnable(Provide(func() ReadinessRunnable {
						defZ()
						return runnableY
					})()).Tag(101)
					return nil
				})
				require.NoError(t, err)
				rrImpl, ok := rr.(*runner)
				require.True(t, ok)

				runnableOrderRequirements{
					{
						tag: 0,
					},
					{
						tag:       21,
						dependsOn: []any{0},
					},
					{
						tag:       22,
						dependsOn: []any{0},
					},
					{
						tag:       100,
						dependsOn: []any{21, 22, 102},
					},
					{
						tag:       101,
						dependsOn: []any{21, 22, 102},
					},
					{
						tag: 102,
					},
				}.CheckLcNodesGraph(t, rrImpl.graph)
				require.Empty(t, globalRegistry.pendingNodes)
			})
		})

		t.Run("errors and panics", func(t *testing.T) {
			userPanicValue := "userPanicValue"

			t.Run("nil provider function", func(t *testing.T) {
				t.Cleanup(testsCleanup)
				tests.RequirePanicsWithErrorIs(t, func() {
					Provide[int](nil)
				}, errNilProviderFn)
				tests.RequirePanicsWithErrorIs(t, func() {
					ProvideE[int](nil)
				}, errNilProviderFn)
				tests.RequirePanicsWithErrorIs(t, func() {
					_ = NewRunner(nil)
				}, errNilProviderFn)
				_, err := NewRunnerE(nil)
				require.ErrorIs(t, err, errNilProviderFn)
			})

			t.Run("simple ProvideE errors", func(t *testing.T) {
				callOrdersSet := [][]int{
					{1, 2, 3},
					{1, 3, 2},
					{2, 1, 3},
					{2, 3, 1},
					{3, 1, 2},
					{3, 2, 1},
				}
				for _, callsOrder := range callOrdersSet {
					t.Run(fmt.Sprintf("calls order: %v", callsOrder), func(t *testing.T) {
						t.Cleanup(testsCleanup)
						expDef1Err := errors.New("def1 error")
						def1 := ProvideE(func() (int, error) {
							return 1, expDef1Err
						})
						def1ErrMsgRequirements := new(errMsgRequirement).
							Raw(expDef1Err.Error())

						def2 := ProvideE(func() (int, error) {
							_, err := def1()
							return 2, fmt.Errorf("wrappedInDef2: %w", err)
						})
						def2ErrMsgRequirements := new(errMsgRequirement).
							Raw("wrappedInDef2: " + expDef1Err.Error())

						def3 := Provide(func() int {
							UseLateInit(func() {
								_, _ = def2()
							})
							return 3
						})

						for _, defNum := range callsOrder {
							switch defNum {
							case 1:
								_, def1Err := def1()
								require.ErrorIs(t, def1Err, expDef1Err)
								checkErrorMsgLines(t, def1Err, def1ErrMsgRequirements)
							case 2:
								_, def2Err := def2()
								require.ErrorIs(t, def2Err, expDef1Err)
								checkErrorMsgLines(t, def2Err, def2ErrMsgRequirements)
							case 3:
								def3Res := def3()
								require.Equal(t, 3, def3Res)
							}
						}
					})
				}
			})

			t.Run("joined errors", func(t *testing.T) {
				t.Cleanup(testsCleanup)

				def1OwnErr := errors.New("def1 own error")
				def1 := ProvideE(func() (int, error) {
					UseLateInitE(func() error {
						t.Fatalf("should not be called")
						return nil
					})
					return 1, def1OwnErr
				})
				def1Res, def1Err := def1()
				require.Equal(t, 1, def1Res)
				require.Equal(t, def1Err, def1OwnErr)

				def2LateInitErr := errors.New("def2 late init error")
				def2 := Provide(func() int {
					UseLateInitE(func() error {
						return def2LateInitErr
					})
					return 2
				})

				def3OwnErr := errors.New("def3 own error")
				def3 := ProvideE(func() (int, error) {
					return def2() + 10, def3OwnErr
				})
				def3Res, def3Err := def3()
				require.Equal(t, 12, def3Res)
				// the current behavior, can be improved in the future to join foreign lateInit errors
				require.Equal(t, def3Err, def3OwnErr)

				def4OwnErr := errors.New("def4 own error")
				def4 := ProvideE(func() (int, error) {
					UseLateInit(func() {
						panic(userPanicValue)
					})
					return 4, def4OwnErr
				})
				def4Res, def4Err := def4()
				require.Equal(t, 4, def4Res)
				require.Equal(t, def4Err, def4OwnErr)
			})

			t.Run("combined, nested calls", func(t *testing.T) {
				callsOrdersSet := [][]int{
					{1, 2, 3},
					{1, 3, 2},
					{2, 1, 3},
					{2, 3, 1},
					{3, 1, 2},
					{3, 2, 1},
				}
				for _, callsOrder := range callsOrdersSet {
					t.Run(fmt.Sprintf("calls order: %v", callsOrder), func(t *testing.T) {
						t.Cleanup(testsCleanup)

						var def1 = Provide(func() int {
							panic(userPanicValue)
						})
						def1ErrMsgRequirements := new(errMsgRequirement).
							Raw(userPanicValue).
							FileLines(1)

						userFn1 := func() {
							def1()
						}
						userFn2 := func() {
							userFn1()
						}
						userFn3 := func() {
							userFn2()
						}
						def2 := Provide(func() int {
							lateInitAdder := func() {
								UseLateInit(func() {
									userFn3()
								})
							}
							lateInitAdder()
							return 42
						})
						def2ErrMsgRequirements := def1ErrMsgRequirements.
							In().Def(1, "int").
							FileLines(4).
							In().LateInitRegAt().
							FileLines(2)

						panickingDefCallerFn := func() int {
							return def2()
						}
						def3 := Provide(func() int {
							panickingDefCallerFn()
							return 1
						})

						def3ErrMsgRequirements := def2ErrMsgRequirements.
							Of().Def(2, "int").
							FileLines(2)

						for _, defNum := range callsOrder {
							switch defNum {
							case 1:
								tests.RequirePanicsWithErrorAs(
									t,
									func() {
										def1()
									},
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def1ErrMsgRequirements)
										require.Empty(t, globalRegistry.pendingNodes)
									},
								)
							case 2:
								tests.RequirePanicsWithErrorAs(
									t,
									func() {
										def2()
									},
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def2ErrMsgRequirements)
										require.Empty(t, globalRegistry.pendingNodes)
									},
								)
							case 3:
								tests.RequirePanicsWithErrorAs(
									t,
									func() {
										def3()
									},
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def3ErrMsgRequirements)
										require.Empty(t, globalRegistry.pendingNodes)
									},
								)
							}
						}
					})
				}
				t.Cleanup(testsCleanup)

			})

			t.Run("user panic in Provide", func(t *testing.T) {
				callsOrdersSet := [][]int{
					{1, 2, 3, 4},
					{1, 3, 2, 4},
					{2, 1, 3, 4},
					{2, 3, 1, 4},
					{3, 1, 2, 4},
					{3, 2, 1, 4},
				}
				for _, callsOrder := range callsOrdersSet {
					t.Run(fmt.Sprintf("calls order: %v", callsOrder), func(t *testing.T) {
						t.Cleanup(testsCleanup)

						def1 := Provide(func() *readinessRunnableMock {
							panic(userPanicValue)
						})
						def1ErrMsgRequirements := new(errMsgRequirement).
							Raw(userPanicValue).
							FileLines(1)

						def2 := Provide(func() *readinessRunnableMock {
							runnable := newReadinessRunnableMock(2)
							UseLifecycle().AddReadinessRunnable(runnable)
							def1()
							return runnable
						})
						def2ErrMsgRequirements := def1ErrMsgRequirements.
							In().Def(1).
							FileLines(1)

						def3 := ProvideE(func() (*readinessRunnableMock, error) {
							runnable := newReadinessRunnableMock(3)
							UseLifecycle().AddReadinessRunnable(runnable)
							def2()
							return runnable, nil
						})
						def3ErrMsgRequirements := def2ErrMsgRequirements.
							In().Def(2).
							FileLines(1)

						def4 := ProvideE(func() (*readinessRunnableMock, error) {
							UseLateInit(func() {
								def1()
							})
							runnable := newReadinessRunnableMock(4)
							UseLifecycle().AddReadinessRunnable(runnable)
							return runnable, nil
						})
						def4ErrMsgRequirements := def1ErrMsgRequirements.
							In().Def(1).
							FileLines(1).
							In().LateInitRegAt().
							FileLines(1)

						for _, defNum := range callsOrder {
							switch defNum {
							case 1:
								tests.RequirePanicsWithErrorAs(
									t,
									func() { _ = def1() },
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def1ErrMsgRequirements)
									},
								)
							case 2:
								tests.RequirePanicsWithErrorAs(
									t,
									func() { _ = def2() },
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def2ErrMsgRequirements)
									},
								)
							case 3:
								tests.RequirePanicsWithErrorAs(
									t,
									func() { _, _ = def3() },
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def3ErrMsgRequirements)
									},
								)
							case 4:
								tests.RequirePanicsWithErrorAs(
									t,
									func() { _, _ = def4() },
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def4ErrMsgRequirements)
									},
								)
							}
							requireNoRunnableNodes(t)
						}
					})
				}
			})

			t.Run("user panic in ProvideE", func(t *testing.T) {
				callsOrdersSet := [][]int{
					{1, 2, 3, 4},
					{1, 3, 2, 4},
					{2, 1, 3, 4},
					{2, 3, 1, 4},
					{3, 1, 2, 4},
					{3, 2, 1, 4},
				}
				for _, callsOrder := range callsOrdersSet {
					t.Run(fmt.Sprintf("calls order: %v", callsOrder), func(t *testing.T) {
						t.Cleanup(testsCleanup)
						def1 := ProvideE(func() (*readinessRunnableMock, error) {
							UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(1))
							panic(userPanicValue)
						})
						def1ErrMsgRequirements := new(errMsgRequirement).
							Raw(userPanicValue).
							FileLines(1)

						requireNoRunnableNodes(t)

						def2 := Provide(func() *readinessRunnableMock {
							runnable := newReadinessRunnableMock(2)
							UseLifecycle().AddReadinessRunnable(runnable)
							_, _ = def1()
							t.Fatal("should not reach here")
							return runnable
						})
						def2ErrMsgRequirements := def1ErrMsgRequirements.
							In().DefErr(1).
							FileLines(1)

						def3 := ProvideE(func() (*readinessRunnableMock, error) {
							runnable := newReadinessRunnableMock(3)
							UseLifecycle().AddReadinessRunnable(runnable)
							_, _ = def1()
							t.Fatal("should not reach here")
							return runnable, nil
						})
						def3ErrMsgRequirements := def1ErrMsgRequirements.
							In().DefErr(1).
							FileLines(1)

						def4 := ProvideE(func() (*readinessRunnableMock, error) {
							UseLateInit(func() {
								_, _ = def1()
							})
							runnable := newReadinessRunnableMock(4)
							UseLifecycle().AddReadinessRunnable(runnable)
							return runnable, nil
						})
						def4ErrMsgRequirements := def1ErrMsgRequirements.
							In().DefErr(1).
							FileLines(1).
							In().LateInitRegAt().
							FileLines(1)

						for _, defNum := range callsOrder {
							switch defNum {
							case 1:
								tests.RequirePanicsWithErrorAs(
									t,
									func() { _, _ = def1() },
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def1ErrMsgRequirements)
									},
								)
							case 2:
								tests.RequirePanicsWithErrorAs(
									t,
									func() { _ = def2() },
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def2ErrMsgRequirements)
									},
								)
							case 3:
								tests.RequirePanicsWithErrorAs(
									t,
									func() { _, _ = def3() },
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def3ErrMsgRequirements)
									},
								)
							case 4:
								tests.RequirePanicsWithErrorAs(
									t,
									func() { _, _ = def4() },
									func(v errDepRegFailed) {
										require.True(t, v.hasUserCodePanic)
										checkErrorMsgLines(t, v, def4ErrMsgRequirements)
									},
								)
							}
						}
						requireNoRunnableNodes(t)
					})
				}
			})

			t.Run("user error in ProvideE", func(t *testing.T) {
				t.Cleanup(testsCleanup)
				defErr := errors.New("error in ProvideE")
				def1 := ProvideE(func() (*readinessRunnableMock, error) {
					UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(1))
					return nil, defErr
				})
				def2Res := newReadinessRunnableMock(2)
				def2 := Provide(func() *readinessRunnableMock {
					UseLifecycle().AddReadinessRunnable(def2Res).Tag(2)
					_, err := def1()
					require.ErrorIs(t, err, defErr)
					return def2Res
				})
				_, err := def1()
				requireNoRunnableNodes(t)
				require.ErrorIs(t, err, defErr)
				require.Len(t, createdNodes, 2)
				require.Empty(t, createdNodes[1].GetLifecycleHooks())
				require.Equal(t, def2Res, def2())
				require.Equal(t, 2, createdNodes[2].GetLifecycleHooks()[0].tag)
			})

			t.Run("LateInitErr in ProvideE", func(t *testing.T) {
				callsOrdersSet := [][]int{
					{1, 2},
					{2, 1},
				}
				for _, callsOrder := range callsOrdersSet {
					t.Run(fmt.Sprintf("calls order: %v", callsOrder), func(t *testing.T) {
						t.Cleanup(testsCleanup)
						liErr := errors.New("error in LateInit")

						isDef1Called := false
						def1 := ProvideE(func() (*readinessRunnableMock, error) {
							UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(11))
							UseLateInitE(func() error {
								UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(12))
								return liErr
							})
							return nil, nil
						})
						def1ErrMsgRequirements := new(errMsgRequirement).
							Raw(liErr.Error()).
							In().LateInitRegAt().
							FileLines(1)

						runnable2 := newReadinessRunnableMock(2)
						def2 := Provide(func() *readinessRunnableMock {
							UseLifecycle().AddReadinessRunnable(runnable2).Tag(2)
							_, err := def1()
							if isDef1Called {
								require.ErrorIs(t, err, liErr)
								checkErrorMsgLines(t, err, def1ErrMsgRequirements)
							} else {
								require.NoError(t, err)
								_, err = def1()
								require.NoError(t, err)
							}
							// ignore err, our node should remain valid
							return runnable2
						})
						def2ErrMsgRequirements := def1ErrMsgRequirements.
							Of().DefErr(1).
							FileLines(1)

						for _, defNum := range callsOrder {
							switch defNum {
							case 1:
								_, def1Err := def1()
								isDef1Called = true
								checkErrorMsgLines(t, def1Err, def1ErrMsgRequirements)
							case 2:
								if isDef1Called {
									def2Value := def2()
									require.Equal(t, runnable2, def2Value)
									require.Len(t, createdNodes, 2)
									require.Equal(t, 2, createdNodes[2].GetLifecycleHooks()[0].tag)
									require.Equal(t, nodeRegStateWithLcHooks, createdNodes[2].GetRegState())
								} else {
									tests.RequirePanics(t, func() {
										_ = def2()
									}, func(r any) {
										rAsErr, ok := r.(error)
										require.True(t, ok)
										checkErrorMsgLines(t, rAsErr, def2ErrMsgRequirements)
									})
									requireNoRunnableNodes(t)
								}
							}
						}
					})
				}
			})

			t.Run("cyclic testDep in defs", func(t *testing.T) {
				callOrders := [][]int{
					{1, 2, 3, 4},
					{1, 3, 2, 4},
					{2, 1, 3, 4},
					{2, 3, 1, 4},
					{3, 1, 2, 4},
					{3, 2, 1, 4},
					{4, 1, 2, 3},
					{4, 2, 1, 3},
				}
				for _, callOrder := range callOrders {
					t.Run(fmt.Sprintf("call order: %v", callOrder), func(t *testing.T) {
						t.Cleanup(testsCleanup)

						var def1, def2, def3, def4 func() int

						def1ErrMsgRequirements := new(errMsgRequirement).
							CyclicDependency().
							Arrow().Def(1, "int").
							FileLines(1).
							In().Def(2, "int").
							FileLines(1).
							In().Def(1, "int")

						def2ErrMsgRequirements := new(errMsgRequirement).
							CyclicDependency().
							Arrow().Def(2, "int").
							FileLines(1).
							In().Def(1, "int").
							FileLines(1).
							In().Def(2, "int")

						def3ErrMsgRequirements := def2ErrMsgRequirements.
							FileLines(1)

						def4ErrMsgRequirements := def2ErrMsgRequirements.
							FileLines(1).
							In().LateInitRegAt().
							FileLines(1)

						// 3 -> 2 <-> 1
						// 4 -> 2 <-> 1
						def1 = Provide(func() int {
							def2()
							return 1
						})
						def2 = Provide(func() int {
							def1()
							return 2
						})
						def3 = Provide(func() int {
							def2()
							return 3
						})
						def4 = Provide(func() int {
							UseLateInit(func() {
								def2()
							})
							return 4
						})

						for _, callIdx := range callOrder {
							switch callIdx {
							case 1:
								tests.RequirePanicsWithErrorIs(
									t,
									func() {
										_ = def1()
									},
									ErrCyclicDependency,
									func(err error) {
										checkErrorMsgLines(t, err, def1ErrMsgRequirements)
									},
								)
							case 2:
								tests.RequirePanicsWithErrorIs(
									t,
									func() {
										_ = def2()
									},
									ErrCyclicDependency,
									func(err error) {
										checkErrorMsgLines(t, err, def2ErrMsgRequirements)
									},
								)
							case 3:
								tests.RequirePanicsWithErrorIs(
									t,
									func() {
										_ = def3()
									},
									ErrCyclicDependency,
									func(err error) {
										checkErrorMsgLines(t, err, def3ErrMsgRequirements)
									},
								)
							case 4:
								tests.RequirePanicsWithErrorIs(
									t,
									func() {
										_ = def4()
									},
									ErrCyclicDependency,
									func(err error) {
										checkErrorMsgLines(t, err, def4ErrMsgRequirements)
									},
								)
							}
						}
					})
				}
			})

			t.Run("cyclic testDep hidden behind ProvideE", func(t *testing.T) {
				callsOrdersSet := [][]int{
					{1, 2, 3},
					{1, 3, 2},
					{2, 1, 3},
					{2, 3, 1},
					{3, 1, 2},
					{3, 2, 1},
				}
				for _, callsOrder := range callsOrdersSet {
					t.Run(fmt.Sprintf("calls order: %v", callsOrder), func(t *testing.T) {
						t.Cleanup(testsCleanup)
						var def1 func() int
						def1 = Provide(func() int {
							return def1()
						})
						def2 := ProvideE(func() (int, error) {
							return def1(), nil
						})
						def3 := Provide(func() int {
							UseLateInit(func() {
								_ = def1()
							})
							return 2
						})
						for _, defNum := range callsOrder {
							switch defNum {
							case 1:
								tests.RequirePanics(t, func() {
									_ = def1()
								}, func(r any) {
									rAsErr, ok := r.(error)
									require.True(t, ok)
									require.ErrorIs(t, rAsErr, ErrCyclicDependency)
								})
							case 2:
								def2Value, def2Err := def2()
								require.Equal(t, 0, def2Value)
								require.ErrorIs(t, def2Err, ErrCyclicDependency)
							case 3:
								tests.RequirePanicsWithErrorIs(t, func() {
									_ = def3()
								}, ErrCyclicDependency)
							}
						}
					})
				}
			})
		})
	})
}
