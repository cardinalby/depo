package depo

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cardinalby/depo/internal/tests"
	"github.com/stretchr/testify/require"
)

func TestNewRunner(t *testing.T) {
	t.Run("NewRunnerE called in provider context", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		def1 := ProvideE(func() (int, error) {
			runnerCaller := func() (Runner, error) {
				return NewRunnerE(func() error {
					return nil
				})
			}
			_, err := runnerCaller()
			require.ErrorIs(t, err, errInProvideContext)
			return 1, err
		})
		def1ErrMsgRequirements := new(errMsgRequirement).
			Raw("NewRunnerE " + errInProvideContext.Error()).
			FileLines(2)
		_, def1Err := def1()
		checkErrorMsgLines(t, def1Err, def1ErrMsgRequirements)

		def2 := Provide(func() int {
			UseLateInitE(func() error {
				_, err := NewRunnerE(func() error {
					return nil
				})
				return err
			})
			return 2
		})

		def2ErrMsgRequirements := new(errMsgRequirement).
			Raw("NewRunnerE " + errInProvideContext.Error()).
			FileLines(1).
			In().LateInitRegAt().
			FileLines(1)
		tests.RequirePanicsWithErrorIs(
			t,
			func() {
				def2()
			},
			errInProvideContext,
			func(err error) {
				checkErrorMsgLines(t, err, def2ErrMsgRequirements)
			})
	})

	t.Run("runnables circular dependency", func(t *testing.T) {
		t.Run("len 0", func(t *testing.T) {
			t.Cleanup(testsCleanup)
			// we allow direct dependency on itself since from runnables point of view it's the same as if
			// it depends on itself through non-runnable nodes (which is allowed, it doesn't prevent us from
			// determining the proper start/shutdown order)
			var def1 func() int
			def1 = Provide(func() int {
				UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(1)).Tag(1)
				UseLateInit(func() {
					def1()
				})
				return 1
			})
			r, err := NewRunnerE(func() error {
				require.Equal(t, 1, def1())
				return nil
			})
			require.NoError(t, err)
			require.NotNil(t, r)
		})

		t.Run("len 1", func(t *testing.T) {
			t.Cleanup(testsCleanup)
			var def1, def2 func() int
			def1 = Provide(func() int {
				UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(1)).Tag(1)
				UseLateInit(func() {
					def2()
				})
				return 1
			})
			def2 = Provide(func() int {
				UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(2)).Tag(2)
				return def1() + 10
			})
			_, err := NewRunnerE(func() error {
				require.Equal(t, 11, def2())
				return nil
			})
			require.ErrorIs(t, err, ErrCyclicDependency)
			def2ErrMsgRequirements := new(errMsgRequirement).
				Raw(ErrCyclicDependency.Error()+" between components with UseLifecycle:").
				Arrow().Def(2, "int").
				Padding(lnFrameNamePrefixLen).Def(1, "int").
				Arrow().Def(2, "int")
			checkErrorMsgLines(t, err, def2ErrMsgRequirements)
		})

		t.Run("len 3", func(t *testing.T) {
			t.Cleanup(testsCleanup)
			var def1, def2, def3 func() int
			def1 = Provide(func() int {
				UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(1)).Tag(1)
				UseLateInit(func() {
					def3()
				})
				return 1
			})
			def2 = Provide(func() int {
				return def1() + 10
			})
			def3 = Provide(func() int {
				UseLifecycle().AddReadinessRunnable(newReadinessRunnableMock(3)).Tag(3)
				return def2() + 100
			})
			def4 := Provide(func() int {
				return def3() + 1000
			})
			_, err := NewRunnerE(func() error {
				require.Equal(t, 1111, def4())
				return nil
			})
			require.ErrorIs(t, err, ErrCyclicDependency)

			def3ErrMsgRequirements := new(errMsgRequirement).
				Raw(ErrCyclicDependency.Error()+" between components with UseLifecycle:").
				Arrow().Def(3, "int").
				Padding(lnFrameNamePrefixLen).Def(1, "int").
				Padding(lnFrameNamePrefixLen).Def(2, "int").
				Arrow().Def(3, "int").
				Padding(lnFrameNamePrefixLen).Def(4, "int")
			checkErrorMsgLines(t, err, def3ErrMsgRequirements)
		})
	})

	t.Run("already running", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		def1 := Provide(func() int {
			UseLifecycle().AddReadinessRunFn(func(ctx context.Context, onReady func()) error {
				onReady()
				<-ctx.Done()
				return ctx.Err()
			})
			return 1
		})
		r, err := NewRunnerE(func() error {
			def1()
			return nil
		})
		require.NoError(t, err)
		var firstRunDone atomic.Bool
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			require.ErrorIs(t, r.Run(ctx, nil), context.Canceled)
			firstRunDone.Store(true)
		}()
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
		err = r.Run(context.Background(), nil)
		require.ErrorIs(t, err, ErrAlreadyRunning)
		time.Sleep(10 * time.Millisecond)
		require.False(t, firstRunDone.Load())
		cancel()
		time.Sleep(10 * time.Millisecond)
		require.True(t, firstRunDone.Load())
	})
}
