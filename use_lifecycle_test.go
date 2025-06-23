package depo

import (
	"context"
	"errors"
	"testing"

	"github.com/cardinalby/depo/internal/tests"
	"github.com/stretchr/testify/require"
)

func TestUseLifecycle_NotInProvideContext(t *testing.T) {
	t.Cleanup(testsCleanup)

	tests.RequirePanicsWithErrorIs(t, func() {
		UseLifecycle()
	}, errNotInProviderFn)

	def1 := Provide(func() int {
		done := make(chan struct{})
		go func() {
			defer close(done)
			tests.RequirePanicsWithErrorIs(t, func() {
				UseLifecycle()
			}, errNotInProviderFn)
		}()
		<-done
		return 1
	})
	def1()
}

func TestUseLifecycle_AddReadinessRunnable(t *testing.T) {
	t.Cleanup(testsCleanup)

	t.Run("valid case", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		runnable := newReadinessRunnableMock(1)
		def1 := Provide(func() int {
			UseLifecycle().AddReadinessRunnable(runnable).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that the runnable was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
	})

	t.Run("nil lcRoles", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddReadinessRunnable(nil)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errNilValue)
	})

	t.Run("already added lcRoles", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		runnable1 := newRunnableMock(1)
		runnable2 := newReadinessRunnableMock(2)
		def1 := Provide(func() int {
			UseLifecycle().AddRunnable(runnable1).AddReadinessRunnable(runnable2)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errAlreadyAdded)
	})
}

func TestUseLifecycle_AddReadinessRunFn(t *testing.T) {
	t.Cleanup(testsCleanup)

	t.Run("valid case", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddReadinessRunFn(func(ctx context.Context, onReady func()) error {
				onReady()
				<-ctx.Done()
				return ctx.Err()
			}).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that the runnable was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
	})

	t.Run("nil function", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddReadinessRunFn(nil)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errNilValue)
	})
}

func TestUseLifecycle_AddRunnable(t *testing.T) {
	t.Cleanup(testsCleanup)

	t.Run("valid case", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		runnable := newRunnableMock(1)
		def1 := Provide(func() int {
			UseLifecycle().AddRunnable(runnable).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that the runnable was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
	})

	t.Run("nil lcRoles", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddRunnable(nil)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errNilValue)
	})

	t.Run("already added readiness lcRoles", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		readinessRunnable := newReadinessRunnableMock(1)
		runnable := newRunnableMock(2)
		def1 := Provide(func() int {
			UseLifecycle().AddReadinessRunnable(readinessRunnable).AddRunnable(runnable)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errAlreadyAdded)
	})
}

func TestUseLifecycle_AddRunFn(t *testing.T) {
	t.Cleanup(testsCleanup)

	t.Run("valid case", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddRunFn(func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			}).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that the runnable was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
	})

	t.Run("nil function", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddRunFn(nil)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errNilValue)
	})
}

func TestUseLifecycle_AddStarter(t *testing.T) {
	t.Cleanup(testsCleanup)

	t.Run("valid case", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		starter := newStarterMock(1)
		def1 := Provide(func() int {
			UseLifecycle().AddStarter(starter).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that the runnable was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
		require.Equal(t, starter, lcHooks[0].starter)
	})

	t.Run("nil starterMock", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddStarter(nil)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errNilValue)
	})

	t.Run("already added starterMock", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		starter1 := newStarterMock(1)
		starter2 := newStarterMock(2)
		def1 := Provide(func() int {
			UseLifecycle().AddStarter(starter1).AddStarter(starter2)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errAlreadyAdded)
	})

	t.Run("conflict with lcRoles", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		runnable := newRunnableMock(1)
		starter := newStarterMock(2)
		def1 := Provide(func() int {
			UseLifecycle().AddRunnable(runnable).AddStarter(starter)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errAlreadyAdded)
	})
}

func TestUseLifecycle_AddStartFn(t *testing.T) {
	t.Cleanup(testsCleanup)

	t.Run("valid case", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddStartFn(func(ctx context.Context) error {
				return nil
			}).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that the runnable was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
		require.NotNil(t, lcHooks[0].starter)
	})

	t.Run("nil function", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddStartFn(nil)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errNilValue)
	})
}

func TestUseLifecycle_AddCloser(t *testing.T) {
	t.Cleanup(testsCleanup)

	t.Run("valid case", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		closer := newCloserMock(1)
		def1 := Provide(func() int {
			UseLifecycle().AddCloser(closer).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that the runnable was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
		require.Equal(t, closer, lcHooks[0].closer)
	})

	t.Run("nil closer", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddCloser(nil)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errNilValue)
	})

	t.Run("already added closer", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		closer1 := newCloserMock(1)
		closer2 := newCloserMock(2)
		def1 := Provide(func() int {
			UseLifecycle().AddCloser(closer1).AddCloser(closer2)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errAlreadyAdded)
	})

	t.Run("conflict with lcRoles", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		runnable := newRunnableMock(1)
		closer := newCloserMock(2)
		def1 := Provide(func() int {
			UseLifecycle().AddRunnable(runnable).AddCloser(closer)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errAlreadyAdded)
	})
}

func TestUseLifecycle_AddCloseFn(t *testing.T) {
	t.Cleanup(testsCleanup)

	t.Run("valid case", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddCloseFn(func() {
				// do nothing
			}).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that the runnable was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
		require.NotNil(t, lcHooks[0].closer)
	})

	t.Run("nil function", func(t *testing.T) {
		t.Cleanup(testsCleanup)

		def1 := Provide(func() int {
			UseLifecycle().AddCloseFn(nil)
			return 1
		})

		tests.RequirePanicsWithErrorIs(t, func() {
			def1()
		}, errNilValue)
	})
}

func TestUseLifecycle_CombinedLifecyclePhases(t *testing.T) {
	t.Run("only starterMock", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		starter := newStarterMock(1)

		def1 := Provide(func() int {
			UseLifecycle().AddStarter(starter).Tag(1)
			return 1
		})

		result := def1()
		require.Equal(t, 1, result)

		// Check that only starter was registered
		require.Len(t, createdNodes, 1)
		lcHooks := createdNodes[1].GetLifecycleHooks()
		require.Len(t, lcHooks, 1)
		require.Equal(t, 1, lcHooks[0].tag)
		require.Equal(t, starter, lcHooks[0].starter)
		require.Nil(t, lcHooks[0].waiter)
		require.Nil(t, lcHooks[0].closer)
	})
}

func TestUseLifecycle_MultipleHooks(t *testing.T) {
	t.Cleanup(testsCleanup)

	runnable1 := newReadinessRunnableMock(1)
	runnable2 := newReadinessRunnableMock(2)

	def1 := Provide(func() int {
		UseLifecycle().AddReadinessRunnable(runnable1).Tag(1)
		UseLifecycle().AddReadinessRunnable(runnable2).Tag(2)
		return 1
	})

	result := def1()
	require.Equal(t, 1, result)

	// Check that both runnables were registered
	require.Len(t, createdNodes, 1)
	lcHooks := createdNodes[1].GetLifecycleHooks()
	require.Len(t, lcHooks, 2)

	// Check that both tags are present (order might vary)
	tags := []any{lcHooks[0].tag, lcHooks[1].tag}
	require.Contains(t, tags, 1)
	require.Contains(t, tags, 2)
}

func TestUseLifecycle_TaggedLifecycle(t *testing.T) {
	t.Cleanup(testsCleanup)

	type customTag struct {
		name string
		id   int
	}

	tag := customTag{name: "test", id: 42}
	runnable := newReadinessRunnableMock(1)

	def1 := Provide(func() int {
		UseLifecycle().AddReadinessRunnable(runnable).Tag(tag)
		return 1
	})

	result := def1()
	require.Equal(t, 1, result)

	// Check that the custom tag was set
	require.Len(t, createdNodes, 1)
	lcHooks := createdNodes[1].GetLifecycleHooks()
	require.Len(t, lcHooks, 1)
	require.Equal(t, tag, lcHooks[0].tag)
}

func TestUseLifecycle_NoLifecycleAdded(t *testing.T) {
	t.Cleanup(testsCleanup)

	def1 := Provide(func() int {
		UseLifecycle().Tag(1) // Only tag, no lifecycle phases
		return 1
	})

	result := def1()
	require.Equal(t, 1, result)

	// Check that no runnable traits were registered since no lifecycle phases were added
	require.Len(t, createdNodes, 1)
	lcHooks := createdNodes[1].GetLifecycleHooks()
	require.Empty(t, lcHooks)
}

func TestUseLifecycle_ErrorPropagation(t *testing.T) {
	t.Cleanup(testsCleanup)

	testErr := errors.New("test error")

	def1 := ProvideE(func() (int, error) {
		UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(1)).Tag(1)
		return 1, testErr
	})

	result, err := def1()
	require.Equal(t, 1, result)
	require.ErrorIs(t, err, testErr)

	// Check that the component was registered but marked as failed
	require.Len(t, createdNodes, 1)
	require.Equal(t, nodeRegStateWithNoLcHooks, createdNodes[1].GetRegState())
	// Runnable traits should still be empty because the component failed
	lcHooks := createdNodes[1].GetLifecycleHooks()
	require.Empty(t, lcHooks)
}
