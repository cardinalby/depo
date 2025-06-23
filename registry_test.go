package depo

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cardinalby/depo/internal/dep"
	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/tests"
	"github.com/stretchr/testify/require"
)

func TestRegistryDepId_IsUnique(t *testing.T) {
	t.Run("concurrent creation", func(t *testing.T) {
		registry := newRegistry()
		callerCtx := runtm.NewCallerCtx(0)
		depCtxs := make([]dep.Ctx, 100)
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				depCtxs[i] = registry.NewDepCtx(callerCtx)
			}()
		}
		wg.Wait()
		uniqueDepCtxs := make(map[dep.Ctx]struct{})
		for i := 0; i < 100; i++ {
			if _, exists := uniqueDepCtxs[depCtxs[i]]; exists {
				t.Errorf("Duplicate Ctx found: %s", depCtxs[i])
			}
			uniqueDepCtxs[depCtxs[i]] = struct{}{}
		}
	})

	t.Run("loop", func(t *testing.T) {
		registry := newRegistry()
		ids := make([]dep.Ctx, 2)
		for i := 0; i < 2; i++ {
			// call at the same line
			ids[i] = registry.NewDepCtx(runtm.NewCallerCtx(0))
		}
		if ids[0] == ids[1] {
			t.Errorf("%s == %s", ids[0], ids[1])
		}
	})

	t.Run("only 1 active root provideFn", func(t *testing.T) {
		var wg sync.WaitGroup
		var isDef3Providing atomic.Bool
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				def1 := Provide(func() int {
					return 1
				})
				def2 := Provide(func() int {
					UseLateInit(func() {})
					return 2
				})
				def3 := Provide(func() int {
					if isDef3Providing.Swap(true) {
						t.Error("Def3 is already providing")
					}
					defer func() {
						if !isDef3Providing.Swap(false) {
							t.Error("Def3 is not providing")
						}
					}()
					UseLateInit(func() {
						def1()
					})
					return def2() + 1
				})
				for i := 0; i < 10; i++ {
					def3()
				}
			}()
		}
		wg.Wait()
	})

	t.Run("consistent stack on panic in provideFn", func(t *testing.T) {
		testPanicMsg := "test panic"

		callsOrderSet := [][]int{
			{1, 2},
			{2, 1},
			{2},
		}
		for _, callsOrder := range callsOrderSet {
			t.Run(fmt.Sprintf("callsOrder %v", callsOrder), func(t *testing.T) {
				t.Cleanup(testsCleanup)

				def1 := Provide(func() int {
					UseLateInit(func() {})
					panic(testPanicMsg)
				})
				def2 := Provide(func() int {
					UseLateInit(func() {})
					def1()
					return 2
				})

				for _, call := range callsOrder {
					switch call {
					case 1:
						tests.RequirePanicsWithErrorAs[errDepRegFailed](
							t,
							func() { def2() },
						)
						require.True(t, globalRegistry.nodeFrames.IsEmpty())
					case 2:
						tests.RequirePanicsWithErrorAs[errDepRegFailed](
							t,
							func() { def2() },
						)
						require.True(t, globalRegistry.nodeFrames.IsEmpty())
					}
				}

			})
		}
	})
}
