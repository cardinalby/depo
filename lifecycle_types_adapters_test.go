package depo

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFnReadinessRunnable_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var receivedCtx context.Context
	var hasReceivedOnReady atomic.Bool
	fnErr := errors.New("test error")
	fn := &fnReadinessRunnable{fn: func(ctx context.Context, onReady func()) (err error) {
		receivedCtx = ctx
		onReady()
		return fnErr
	}}
	require.Equal(t, fnErr, fn.Run(ctx, func() {
		hasReceivedOnReady.Store(true)
	}))
	require.Equal(t, ctx, receivedCtx)
	require.True(t, hasReceivedOnReady.Load())
	m := make(map[ReadinessRunnable]any)
	m[fn] = nil // ensure is hashable
}

func TestFnRunnable_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var receivedCtx context.Context
	fnErr := errors.New("test error")
	fn := &fnRunnable{fn: func(ctx context.Context) (err error) {
		receivedCtx = ctx
		return fnErr
	}}
	require.Equal(t, fnErr, fn.Run(ctx))
	require.Equal(t, ctx, receivedCtx)
	m := make(map[Runnable]any)
	m[fn] = nil // ensure is hashable
}

func TestFnStarter_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var receivedCtx context.Context
	fnErr := errors.New("test error")
	fn := &fnStarter{fn: func(ctx context.Context) (err error) {
		receivedCtx = ctx
		return fnErr
	}}
	require.Equal(t, fnErr, fn.Start(ctx))
	require.Equal(t, ctx, receivedCtx)
	m := make(map[Starter]any)
	m[fn] = nil // ensure is hashable
}

func TestFnCloser_Close(t *testing.T) {
	isCalled := false
	fn := &fnCloser{fn: func() {
		isCalled = true
	}}
	fn.Close()
	require.True(t, isCalled)
}

func TestFnWaiter_Wait(t *testing.T) {
	fnErr := errors.New("test error")
	fn := &fnWaiter{fn: func() (err error) {
		return fnErr
	}}
	require.Equal(t, fnErr, fn.wait())
	m := make(map[waiter]any)
	m[fn] = nil // ensure is hashable
}

func TestPhasedReadinessRunnable(t *testing.T) {
	prepare := func() (*readinessRunnableMock, *phasedReadinessRunnable, context.Context, context.CancelFunc) {
		runnable := newReadinessRunnableMock(0)
		phased := newPhasedReadinessRunnable(runnable)
		ctx, cancel := context.WithCancel(context.Background())
		return runnable, phased, ctx, cancel
	}

	startErr := errors.New("start error")
	waitErr := errors.New("wait error")
	closeErr := errors.New("close error")

	t.Run("Start with done ctx", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		mRunnable, phased, ctx, cancel := prepare()
		cancel()
		err := phased.Start(ctx)
		require.ErrorIs(t, err, ctx.Err())
		require.Positive(t, mRunnable.starterMock.enterEventId.Load())
		require.Positive(t, mRunnable.starterMock.exitEventId.Load())
		require.Negative(t, mRunnable.waiterMock.enterEventId.Load())
	})

	t.Run("Start err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		mRunnable, phased, ctx, cancel := prepare()
		defer cancel()
		go func() {
			mRunnable.starterMock.errChan <- startErr
		}()
		require.ErrorIs(t, phased.Start(ctx), startErr)
		require.Positive(t, mRunnable.starterMock.enterEventId.Load())
		require.Positive(t, mRunnable.starterMock.exitEventId.Load())
		require.Negative(t, mRunnable.waiterMock.enterEventId.Load())
	})

	t.Run("cancel ctx after Start, close err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			go func() {
				mRunnable.starterMock.errChan <- nil
				for mRunnable.waiterMock.enterEventId.Load() < 0 {
					time.Sleep(10 * time.Second)
				}
				phased.close(nil)
				time.Sleep(10 * time.Second)
				mRunnable.closerMock.errChan <- closeErr
			}()
			require.NoError(t, phased.Start(ctx))
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			require.Positive(t, mRunnable.starterMock.exitEventId.Load())
			time.Sleep(5 * time.Second)
			require.Positive(t, mRunnable.waiterMock.enterEventId.Load())

			require.ErrorIs(t, phased.wait(), context.Canceled)
			require.Positive(t, mRunnable.waiterMock.exitEventId.Load())
			require.Positive(t, mRunnable.closerMock.enterEventId.Load())
		})
	})

	t.Run("wait returns nil", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			mRunnable.starterMock.errChan <- nil
			require.NoError(t, phased.Start(ctx))
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			require.Positive(t, mRunnable.starterMock.exitEventId.Load())
			time.Sleep(5 * time.Second)
			require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				mRunnable.waiterMock.errChan <- nil
			}()
			require.NoError(t, phased.wait(), waitErr)
			require.Positive(t, mRunnable.waiterMock.exitEventId.Load())
			require.Negative(t, mRunnable.closerMock.enterEventId.Load())
			phased.close(closeErr)
			require.Negative(t, mRunnable.closerMock.enterEventId.Load())
		})
	})

	t.Run("wait err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			mRunnable.starterMock.errChan <- nil
			require.NoError(t, phased.Start(ctx))
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				require.Negative(t, mRunnable.closerMock.enterEventId.Load())
				mRunnable.waiterMock.errChan <- waitErr
			}()
			require.ErrorIs(t, phased.wait(), waitErr)
		})
	})

	t.Run("close with no err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			mRunnable.starterMock.errChan <- nil
			require.NoError(t, phased.Start(ctx))
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			require.Positive(t, mRunnable.starterMock.exitEventId.Load())
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				require.Negative(t, mRunnable.closerMock.enterEventId.Load())
				mRunnable.closerMock.errChan <- nil
				phased.close(nil)
				mRunnable.waiterMock.errChan <- nil
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.closerMock.enterEventId.Load())
			}()
			require.NoError(t, phased.wait())
		})
	})

	t.Run("close with err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			mRunnable.starterMock.errChan <- nil
			require.NoError(t, phased.Start(ctx))
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				require.Negative(t, mRunnable.closerMock.enterEventId.Load())
				mRunnable.closerMock.errChan <- closeErr
				phased.close(closeErr)
				time.Sleep(10 * time.Second)
			}()
			require.ErrorIs(t, phased.wait(), context.Canceled)
			require.Positive(t, mRunnable.closerMock.enterEventId.Load())
			require.Equal(t, closeErr, *mRunnable.waiterMock.ctxDoneCause.Load())
		})
	})
}

func TestPhasedRunnable(t *testing.T) {
	prepare := func() (*runnableMock, *phasedRunnable, context.Context, context.CancelFunc) {
		runnable := newRunnableMock(0)
		phased := newPhasedRunnable(runnable)
		ctx, cancel := context.WithCancel(context.Background())
		return runnable, phased, ctx, cancel
	}

	startErr := errors.New("start error")
	waitErr := errors.New("wait error")
	closeErr := errors.New("close error")

	t.Run("Start with done ctx", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			cancel()
			err := phased.Start(ctx)
			time.Sleep(10 * time.Second)
			require.NoError(t, err) // Start always returns nil for phasedRunnable
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			require.Negative(t, mRunnable.waiterMock.enterEventId.Load())
			phased.close(nil)
			require.ErrorIs(t, phased.wait(), context.Canceled)
		})
	})

	t.Run("Start err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		mRunnable, phased, ctx, cancel := prepare()
		defer cancel()
		go func() {
			mRunnable.starterMock.errChan <- startErr
		}()
		require.NoError(t, phased.Start(ctx)) // Start always returns nil
		require.ErrorIs(t, phased.wait(), startErr)
		require.Positive(t, mRunnable.starterMock.enterEventId.Load())
		require.Negative(t, mRunnable.waiterMock.enterEventId.Load())
		require.Negative(t, mRunnable.waiterMock.enterEventId.Load())
	})

	t.Run("cancel ctx after Start, close err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			go func() {
				mRunnable.starterMock.errChan <- nil
				time.Sleep(10 * time.Second)
				cancel()
			}()
			require.NoError(t, phased.Start(ctx))
			time.Sleep(20 * time.Second)
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			require.Positive(t, mRunnable.starterMock.exitEventId.Load())
			phased.close(nil)
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				require.Positive(t, mRunnable.closerMock.enterEventId.Load())
				mRunnable.closerMock.errChan <- closeErr
			}()
			require.ErrorIs(t, phased.wait(), context.Canceled)
		})
	})

	t.Run("wait returns nil", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			mRunnable.starterMock.errChan <- nil
			require.NoError(t, phased.Start(ctx))
			time.Sleep(10 * time.Second)
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				require.Negative(t, mRunnable.closerMock.enterEventId.Load())
				mRunnable.waiterMock.errChan <- nil
			}()
			require.NoError(t, phased.wait())
			time.Sleep(20 * time.Second)
			phased.close(closeErr)
			require.Negative(t, mRunnable.closerMock.enterEventId.Load())
		})
	})

	t.Run("wait err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			mRunnable.starterMock.errChan <- nil
			require.NoError(t, phased.Start(ctx))
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				require.Negative(t, mRunnable.closerMock.enterEventId.Load())
				mRunnable.waiterMock.errChan <- waitErr
			}()
			require.ErrorIs(t, phased.wait(), waitErr)
		})
	})

	t.Run("close with no err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			mRunnable.starterMock.errChan <- nil
			require.NoError(t, phased.Start(ctx))
			time.Sleep(10 * time.Second)
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				phased.close(nil)
				mRunnable.closerMock.errChan <- nil
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.closerMock.enterEventId.Load())
			}()
			require.ErrorIs(t, phased.wait(), context.Canceled)
		})
	})

	t.Run("close with err", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			mRunnable, phased, ctx, cancel := prepare()
			defer cancel()
			mRunnable.starterMock.errChan <- nil
			require.NoError(t, phased.Start(ctx))
			time.Sleep(10 * time.Second)
			require.Positive(t, mRunnable.starterMock.enterEventId.Load())
			require.Positive(t, mRunnable.starterMock.exitEventId.Load())
			go func() {
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.waiterMock.enterEventId.Load())
				require.Negative(t, mRunnable.closerMock.enterEventId.Load())
				mRunnable.closerMock.errChan <- closeErr
				phased.close(nil)
				time.Sleep(10 * time.Second)
				require.Positive(t, mRunnable.closerMock.enterEventId.Load())
			}()
			require.ErrorIs(t, phased.wait(), context.Canceled)
		})
	})
}
