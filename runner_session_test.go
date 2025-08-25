package depo

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cardinalby/depo/internal/signals"
	"github.com/cardinalby/depo/pkg/contexts"
	"github.com/stretchr/testify/require"
)

type testDefRec struct {
	i                 int
	getComponent      func() int
	starterMock       *starterMock
	waiterMock        *waiterMock
	closerMock        *closerMock
	runnable          Runnable
	readinessRunnable ReadinessRunnable
	tag               int
}

func (r testDefRec) shouldCallCloseAfterWaitDone() bool {
	return r.runnable == nil && r.readinessRunnable == nil
}

type testDefRecs []testDefRec

func (recs testDefRecs) getRunnableTags() (res []int) {
	for _, rec := range recs {
		if rec.runnable != nil {
			res = append(res, rec.tag)
		}
	}
	return res
}

func (recs testDefRecs) setStartErr(err error, duration ...time.Duration) {
	for _, rec := range recs {
		rec.starterMock.errChan <- err
		if len(duration) > 0 {
			rec.starterMock.enterSleepDuration.Store(int64(duration[0]))
		}
	}
}

func (recs testDefRecs) setWaitErr(err error, duration ...time.Duration) {
	for _, rec := range recs {
		rec.waiterMock.errChan <- err
		if len(duration) > 0 {
			rec.waiterMock.enterSleepDuration.Store(int64(duration[0]))
		}
	}
}

func (recs testDefRecs) setCloseErr(err error, duration ...time.Duration) {
	for _, rec := range recs {
		rec.closerMock.errChan <- err
		if len(duration) > 0 {
			rec.closerMock.enterSleepDuration.Store(int64(duration[0]))
		}
	}
}

func (recs testDefRecs) triggerWaitErrOnClose(waitErr error, waitExitSleepD ...time.Duration) {
	for _, rec := range recs {
		if len(waitExitSleepD) > 0 && rec.waiterMock != nil {
			rec.waiterMock.exitSleepDuration.Store(int64(waitExitSleepD[0]))
		}
		if rec.closerMock != nil {
			rec.closerMock.errChan <- nil
		}
		if rec.closerMock != nil {
			rec.closerMock.phaseAction = func() {
				rec.waiterMock.errChan <- waitErr
			}
		}
	}
}

// tag -> order
func (recs testDefRecs) getOrderMapByEventId(getEventId func(testDefRec) int64) map[any]int {
	var clonedRecs []testDefRec
	for _, rec := range recs {
		if getEventId(rec) >= 0 {
			clonedRecs = append(clonedRecs, rec)
		}
	}
	slices.SortFunc(clonedRecs, func(a, b testDefRec) int {
		return int(getEventId(a) - getEventId(b))
	})
	orderMap := make(map[any]int, len(clonedRecs))
	for i, rec := range clonedRecs {
		orderMap[rec.tag] = i
	}
	return orderMap
}

func (recs testDefRecs) hasSleepDurations(getRunnableDuration func(tag any) time.Duration) bool {
	for _, rec := range recs {
		if getRunnableDuration(rec.tag) <= 0 {
			return false
		}
	}
	return true
}

func (recs testDefRecs) requireStarted(
	t *testing.T,
	listener *runnerListenerMock,
	orderReq runnableOrderRequirements,
	notReadyIndexes ...int,
) {
	for i, rec := range recs {
		require.Positive(t, rec.starterMock.enterEventId.Load())
		if !slices.Contains(notReadyIndexes, i) {
			require.Positive(t, rec.starterMock.exitEventId.Load(), "tag %v", rec.tag)
		}
	}
	startEventsOrderMap := listener.getEventsOrderMap(runnerListenerMockEventTypeStart)
	readyEventsOrderMap := listener.getEventsOrderMap(runnerListenerMockEventTypeReady)
	startEnterOrderMap := recs.getOrderMapByEventId(func(r testDefRec) int64 {
		return r.starterMock.enterEventId.Load()
	})
	startExitOrderMap := recs.getOrderMapByEventId(func(r testDefRec) int64 {
		return r.starterMock.exitEventId.Load()
	})
	require.Equal(t, len(startExitOrderMap)+len(notReadyIndexes), len(startEventsOrderMap))
	require.Equal(t, len(startEventsOrderMap)-len(notReadyIndexes), len(readyEventsOrderMap))
	startOrderReq := orderReq
	for _, tag := range recs.getRunnableTags() {
		// runnable have inconsistent start enter and exit event timing:
		// - they start in a goroutine internally but are considered immediately started and unblock dependents
		// - they are considered ready immediately after start and startExit can happen later internally.
		startOrderReq = startOrderReq.RemoveDependenciesOnAndRelink(tag)
	}
	startOrderReq.checkOrderMap(t, startEnterOrderMap)
	startOrderReq.checkOrderMap(t, startExitOrderMap)
	orderReq.checkOrderMap(t, startEventsOrderMap)
	orderReq.checkOrderMap(t, readyEventsOrderMap)

	getEnterSleepDuration := func(tag any) time.Duration {
		return time.Duration(recs[tag.(int)].starterMock.enterSleepDuration.Load())
	}
	if recs.hasSleepDurations(getEnterSleepDuration) {
		orderReq.checkEventTimings(
			t,
			listener.getEvents(runnerListenerMockEventTypeStart),
			func(tag any) time.Duration {
				rec := recs[tag.(int)]
				if rec.runnable != nil {
					return 0 // runnable starts immediately
				}
				return time.Duration(rec.starterMock.enterSleepDuration.Load())
			},
		)
	}
	getExitSleepDuration := func(tag any) time.Duration {
		return time.Duration(recs[tag.(int)].starterMock.exitSleepDuration.Load())
	}
	if recs.hasSleepDurations(getExitSleepDuration) {
		orderReq.checkEventTimings(
			t,
			listener.getEvents(runnerListenerMockEventTypeReady),
			func(tag any) time.Duration {
				return time.Duration(recs[tag.(int)].starterMock.enterSleepDuration.Load())
			},
		)
	}
}

func (recs testDefRecs) requireWaitingNotClosed(
	t *testing.T,
	listener *runnerListenerMock,
) {
	listenerStoppingEvents := listener.getEvents(runnerListenerMockEventTypeStopping)
	for _, rec := range recs {
		require.Positive(t, rec.waiterMock.enterEventId.Load(), "tag %v", rec.tag)
		require.Negative(t, rec.waiterMock.exitEventId.Load(), "tag %v", rec.tag)
		require.Negative(t, rec.closerMock.enterEventId.Load(), "tag %v", rec.tag)
		hasStoppingEvent := slices.ContainsFunc(
			listenerStoppingEvents,
			func(event runnerListenerMockEvent) bool {
				return event.info.Tag() == rec.tag
			},
		)
		require.False(t, hasStoppingEvent, "tag %v", rec.tag)
	}
}

func (recs testDefRecs) requireDoneFullLc(
	t *testing.T,
	listener *runnerListenerMock,
	stopOrderReq runnableOrderRequirements,
) {
	skippedClosesCount := 0
	for _, rec := range recs {
		require.Positive(t, rec.starterMock.enterEventId.Load(), "tag %v", rec.tag)
		require.Positive(t, rec.starterMock.exitEventId.Load(), "tag %v", rec.tag)
		require.Positive(t, rec.waiterMock.enterEventId.Load(), "tag %v", rec.tag)
		require.Positive(t, rec.waiterMock.exitEventId.Load(), "tag %v", rec.tag)
		if rec.shouldCallCloseAfterWaitDone() {
			require.Positive(t, rec.closerMock.enterEventId.Load(), "tag %v", rec.tag)
			require.Positive(t, rec.closerMock.exitEventId.Load(), "tag %v", rec.tag)
		} else {
			closeEnterEventId := rec.closerMock.enterEventId.Load()
			closeExitEventId := rec.closerMock.exitEventId.Load()
			if closeEnterEventId > 0 {
				require.Positive(t, closeExitEventId, "tag %v", rec.tag)
				require.Less(t, closeEnterEventId, closeExitEventId, "tag %v", rec.tag)
				// for runnables close (if has been called) should happen before wait done
				require.Less(t, closeEnterEventId, rec.waiterMock.exitEventId.Load(), "tag %v", rec.tag)
			} else {
				skippedClosesCount++
			}
		}
	}
	closeEnterOrderMap := recs.getOrderMapByEventId(func(r testDefRec) int64 {
		return r.closerMock.enterEventId.Load()
	})
	closeExitOrderMap := recs.getOrderMapByEventId(func(r testDefRec) int64 {
		return r.closerMock.exitEventId.Load()
	})
	doneEventsOrderMap := listener.getEventsOrderMap(runnerListenerMockEventTypeDone)
	stoppingEventsOrderMap := listener.getEventsOrderMap(runnerListenerMockEventTypeStopping)
	require.Equal(t, len(closeEnterOrderMap), len(closeExitOrderMap))
	require.Equal(t, len(closeEnterOrderMap), len(doneEventsOrderMap)-skippedClosesCount)
	if stopOrderReq != nil {
		stopOrderReq.checkOrderMap(t, closeEnterOrderMap)
		stopOrderReq.checkOrderMap(t, closeExitOrderMap)
		stopOrderReq.checkOrderMap(t, doneEventsOrderMap)
		stopOrderReq.checkOrderMap(t, stoppingEventsOrderMap)
	}

	getEnterSleepDuration := func(tag any) time.Duration {
		return time.Duration(recs[tag.(int)].closerMock.enterSleepDuration.Load())
	}
	if recs.hasSleepDurations(getEnterSleepDuration) && stopOrderReq != nil {
		stopOrderReq.checkEventTimings(
			t,
			listener.getEvents(runnerListenerMockEventTypeDone),
			func(tag any) time.Duration {
				return time.Duration(recs[tag.(int)].waiterMock.exitSleepDuration.Load())
			},
		)
	}
}

func (recs testDefRecs) requireDoneOrNotStarted(
	t *testing.T,
	listener *runnerListenerMock,
	stopOrderReq runnableOrderRequirements,
	notStartedTags ...int,
) {
	for _, rec := range recs {
		expectStarted := !slices.Contains(notStartedTags, rec.tag)
		if rec.starterMock.enterEventId.Load() > 0 {
			require.Positive(t, rec.starterMock.exitEventId.Load(),
				"lcRoles %d start entered but not exited", rec.tag,
			)
			require.True(t, expectStarted, "lcRoles %d started but should not be", rec.tag)
		}
		if rec.waiterMock.enterEventId.Load() > 0 {
			if !expectStarted {
				require.Fail(t, "wait entered but node should not have been started",
					"tag: %d", rec.tag,
				)
			}
			require.Positive(t, rec.waiterMock.exitEventId.Load(), "tag %v", rec.tag)
			if rec.shouldCallCloseAfterWaitDone() {
				require.Positive(t, rec.closerMock.enterEventId.Load(), "tag %v", rec.tag)
			}
		}

		if rec.closerMock.enterEventId.Load() > 0 {
			require.Positive(t, rec.closerMock.exitEventId.Load(), "tag %v", rec.tag)
		}
	}
	closeEnterOrderMap := recs.getOrderMapByEventId(func(r testDefRec) int64 {
		return r.closerMock.enterEventId.Load()
	})
	closeExitOrderMap := recs.getOrderMapByEventId(func(r testDefRec) int64 {
		return r.closerMock.exitEventId.Load()
	})
	doneEventsOrderMap := listener.getEventsOrderMap(runnerListenerMockEventTypeDone)
	stoppingEventsOrderMap := listener.getEventsOrderMap(runnerListenerMockEventTypeStopping)

	require.Equal(t, len(closeEnterOrderMap), len(closeExitOrderMap))
	stopOrderReq.checkOrderMap(t, closeEnterOrderMap)
	stopOrderReq.checkOrderMap(t, closeExitOrderMap)
	stopOrderReq.checkOrderMap(t, doneEventsOrderMap)
	stopOrderReq.checkOrderMap(t, stoppingEventsOrderMap)
}

func (recs testDefRecs) exclude(tags ...int) testDefRecs {
	var res testDefRecs
	for _, rec := range recs {
		if !slices.Contains(tags, rec.tag) {
			res = append(res, rec)
		}
	}
	return res
}

func TestRunner_RunAllPhases(t *testing.T) {
	t.Cleanup(testsCleanup)

	//    ╭─> 5 ─> 4 ──┬─> 2 ──> 0
	//    │        ↑   │   ↑     ↑
	// R ─┼─> 8 ─> 7   │   │     │
	//	  │            ↓   │     │
	//    ╰─> 6 ─────> 3 ──┴─────┴──> 1

	type defsVariant struct {
		rootsIndexes     []int
		runnablesIndexes []int
	}
	defsVariants := []defsVariant{
		{
			rootsIndexes: []int{5, 6, 8},
		},
		{
			rootsIndexes: []int{5, 6, 8, 2, 7},
		},
		{
			rootsIndexes: []int{3, 5, 6, 8},
		},
		{
			rootsIndexes:     []int{5, 6, 8},
			runnablesIndexes: []int{5, 1},
		},
		{
			rootsIndexes:     []int{5, 6, 8},
			runnablesIndexes: []int{4},
		},
	}

	testCause := fmt.Errorf("test_cause")
	createDefs := func(
		variant defsVariant,
		runnerOptions ...RunnerOption,
	) (
		recs testDefRecs,
		r Runner,
		startOrderReq runnableOrderRequirements,
		listener *runnerListenerMock,
	) {
		recs = make([]testDefRec, 9)
		for i := range recs {
			recs[i].i = i
			switch {
			case slices.Contains(variant.runnablesIndexes, i):
				runnableMock := newRunnableMock(i)
				recs[i].starterMock = runnableMock.starterMock
				recs[i].waiterMock = runnableMock.waiterMock
				recs[i].closerMock = runnableMock.closerMock
				recs[i].runnable = runnableMock
			default:
				readinessRunnableMock := newReadinessRunnableMock(i)
				recs[i].starterMock = readinessRunnableMock.starterMock
				recs[i].waiterMock = readinessRunnableMock.waiterMock
				recs[i].closerMock = readinessRunnableMock.closerMock
				recs[i].readinessRunnable = readinessRunnableMock
			}
			recs[i].tag = i
		}
		addLcMockLcRoles := func(rec testDefRec) {
			lcBuilder := UseLifecycle()
			switch {
			case rec.runnable != nil:
				lcBuilder.AddRunnable(rec.runnable)
			case rec.readinessRunnable != nil:
				lcBuilder.AddReadinessRunnable(rec.readinessRunnable)
			default:
				if rec.starterMock != nil {
					lcBuilder.AddStarter(rec.starterMock)
				}
				if rec.closerMock != nil {
					lcBuilder.AddCloser(rec.closerMock)
				}
			}
			lcBuilder.Tag(rec.tag)
		}
		recs[0].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[0])
			return 0
		})
		recs[1].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[1])
			return 1
		})
		recs[2].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[2])
			recs[0].getComponent()
			return 2
		})
		recs[3].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[3])
			recs[0].getComponent()
			recs[1].getComponent()
			recs[2].getComponent()
			return 3
		})
		recs[4].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[4])
			recs[2].getComponent()
			recs[3].getComponent()
			return 4
		})
		recs[5].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[5])
			recs[4].getComponent()
			return 5
		})
		recs[6].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[6])
			recs[3].getComponent()
			return 6
		})
		recs[7].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[7])
			recs[4].getComponent()
			return 7
		})
		recs[8].getComponent = Provide(func() int {
			addLcMockLcRoles(recs[8])
			recs[7].getComponent()
			return 8
		})
		listener = &runnerListenerMock{}
		runnerOptions = append(runnerOptions, OptRunnerListeners(listener))
		listener.ShouldBeCalledInCurrentGoroutineId()
		r, err := NewRunnerE(func() error {
			for _, rootIndex := range variant.rootsIndexes {
				recs[rootIndex].getComponent()
			}
			return nil
		}, runnerOptions...)
		require.NoError(t, err)
		rr, ok := r.(*runner)
		require.True(t, ok)
		startOrderReq = runnableOrderRequirements{
			{
				tag: 0,
			},
			{
				tag: 1,
			},
			{
				tag:       2,
				dependsOn: []any{0},
			},
			{
				tag:       3,
				dependsOn: []any{0, 1, 2},
			},
			{
				tag:       4,
				dependsOn: []any{2, 3},
			},
			{
				tag:       5,
				dependsOn: []any{4},
			},
			{
				tag:       6,
				dependsOn: []any{3},
			},
			{
				tag:       7,
				dependsOn: []any{4},
			},
			{
				tag:       8,
				dependsOn: []any{7},
			},
		}
		startOrderReq.CheckLcNodesGraph(t, rr.graph)
		return recs, r, startOrderReq, listener
	}

	t.Run("simple stop on ready", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant)
					ctx, cancel := context.WithCancel(context.Background())
					recs.setStartErr(nil, time.Second*100)
					recs.triggerWaitErrOnClose(context.Canceled, time.Second*100)
					isOnReadyCalled := false
					onReady := func() {
						isOnReadyCalled = true
						recs.requireStarted(t, listener, startOrderReq)
						cancel()
					}
					err := runner.Run(ctx, onReady)
					require.True(t, isOnReadyCalled)
					require.ErrorIs(t, err, context.Canceled)
					recs.requireDoneFullLc(t, listener, startOrderReq.Inverse())
					listener.requireErrors(
						t,
						listenerEventTypeErrRequirements{
							runnerListenerMockEventTypeStopping: errorRequirements{
								"*": context.Canceled,
							},
							runnerListenerMockEventTypeDone: errorRequirements{
								"*": context.Canceled,
							},
							runnerListenerMockEventTypeShutdown: errorRequirements{
								"*": context.Canceled,
							},
						},
					)
				})
			})
		}
	})

	t.Run("simple stop during starts", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant)
					ctx, cancel := context.WithCancel(context.Background())
					recs.setStartErr(nil, time.Second*100)
					onReady := func() {
						require.Fail(t, "onReady should not be called")
					}
					recs.triggerWaitErrOnClose(context.Canceled, time.Second*100)
					go func() {
						time.Sleep(150 * time.Second)
						recs[:3].requireStarted(t, listener, startOrderReq[:3], 2)
						cancel()
					}()
					err := runner.Run(ctx, onReady)
					require.ErrorIs(t, err, context.Canceled)
					recs.requireDoneOrNotStarted(t, listener, startOrderReq.Inverse(), 3, 4, 5, 6, 7, 8)
					listener.requireErrors(
						t,
						listenerEventTypeErrRequirements{
							runnerListenerMockEventTypeStopping: errorRequirements{
								"*": context.Canceled,
							},
							runnerListenerMockEventTypeDone: errorRequirements{
								"*": context.Canceled,
							},
							runnerListenerMockEventTypeShutdown: errorRequirements{
								"*": context.Canceled,
							},
						},
					)
				})
			})
		}
	})

	t.Run("simple stop after ready", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant)
					ctx, cancel := context.WithCancel(context.Background())
					recs.setStartErr(nil, time.Second*100)
					recs.triggerWaitErrOnClose(context.Canceled, time.Second*100)
					isOnReadyCalled := false
					onReady := func() {
						isOnReadyCalled = true
						recs.requireStarted(t, listener, startOrderReq)
						go func() {
							time.Sleep(10 * time.Second)
							cancel()
						}()
					}
					err := runner.Run(ctx, onReady)
					require.True(t, isOnReadyCalled)
					require.ErrorIs(t, err, context.Canceled)
					recs.requireDoneFullLc(t, listener, startOrderReq.Inverse())
					listener.requireErrors(
						t,
						listenerEventTypeErrRequirements{
							runnerListenerMockEventTypeStopping: errorRequirements{
								"*": context.Canceled,
							},
							runnerListenerMockEventTypeDone: errorRequirements{
								"*": context.Canceled,
							},
							runnerListenerMockEventTypeShutdown: errorRequirements{
								"*": context.Canceled,
							},
						},
					)
				})
			})
		}
	})

	t.Run("hanging start", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant)
					ctx, cancel := context.WithCancelCause(context.Background())
					recs.setStartErr(nil, time.Second*100)
					<-recs[8].starterMock.errChan // recs[8] will not start
					recs.triggerWaitErrOnClose(context.Canceled, time.Second*100)
					_ = startOrderReq
					_ = listener
					starterErr := errors.New("starter_err")
					go func() {
						time.Sleep(550 * time.Second)
						recs[:8].requireStarted(t, listener, startOrderReq[:8], 7)
						cancel(testCause)
						time.Sleep(10 * time.Second)
						runtime.Gosched()
						recs[8].starterMock.errChan <- starterErr
					}()
					err := runner.Run(ctx, func() {
						t.Fatal("onReady should not be called")
					})
					require.ErrorIs(t, err, context.Canceled)
					recs.requireDoneOrNotStarted(t, listener, startOrderReq.Inverse())
					listener.requireErrors(
						t,
						listenerEventTypeErrRequirements{
							runnerListenerMockEventTypeStopping: errorRequirements{
								"*": testCause,
							},
							runnerListenerMockEventTypeDone: errorRequirements{
								"*": context.Canceled, 8: func(err error, msgAndArgs []any) {
									require.NotNil(t, err, msgAndArgs...)
								},
							},
							runnerListenerMockEventTypeShutdown: errorRequirements{
								"*": testCause,
							},
						},
					)
				})
			})
		}
	})

	t.Run("wait returns err after ready", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					recs.setStartErr(nil)
					recs.exclude(6).triggerWaitErrOnClose(context.Canceled)
					recs[6:7].triggerWaitErrOnClose(context.Canceled, time.Second*2)
					isOnReadyCalled := false
					onReady := func() {
						isOnReadyCalled = true
						time.Sleep(time.Second)
						recs.requireStarted(t, listener, startOrderReq)
						go func() {
							time.Sleep(time.Second)
							recs[3].waiterMock.errChan <- testCause
							time.Sleep(time.Second)
							require.Positive(t, recs[1].closerMock.enterEventId.Load())
							require.Positive(t, recs[1].closerMock.exitEventId.Load())
							require.Positive(t, recs[1].waiterMock.exitEventId.Load())
						}()
					}
					err := runner.Run(ctx, onReady)
					require.True(t, isOnReadyCalled)
					var componentFailedErr ErrLifecycleHookFailed
					require.ErrorAs(t, err, &componentFailedErr)
					require.Equal(t, any(3), componentFailedErr.LifecycleHook().Tag())
					require.ErrorIs(t, componentFailedErr, testCause)
					listener.requireErrors(
						t,
						listenerEventTypeErrRequirements{
							runnerListenerMockEventTypeStopping: errorRequirements{
								"*": testCause,
							},
							runnerListenerMockEventTypeDone: errorRequirements{
								"*": context.Canceled, 3: testCause,
							},
							runnerListenerMockEventTypeShutdown: errorRequirements{
								"*": testCause,
							},
						},
					)
					stopOrderReq := startOrderReq.Remove(3).Inverse()
					recs.requireDoneFullLc(t, listener, stopOrderReq)
				})
			})
		}
	})

	t.Run("wait returns nil on ready", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					recs.setStartErr(nil)
					recs.triggerWaitErrOnClose(context.Canceled)
					isOnReadyCalled := false
					onReady := func() {
						isOnReadyCalled = true
						time.Sleep(1 * time.Microsecond)
						recs.requireStarted(t, listener, startOrderReq)
						recs[3].waiterMock.errChan <- nil
						go func() {
							time.Sleep(1 * time.Microsecond)
							recs.exclude(3).requireWaitingNotClosed(t, listener)
							listener.requireTagEvent(t, runnerListenerMockEventTypeDone, 3,
								func(event runnerListenerMockEvent) {
									require.Nil(t, event.err)
								},
							)
							recs.exclude(3).setWaitErr(nil)
						}()
					}
					err := runner.Run(ctx, onReady)
					require.True(t, isOnReadyCalled)
					require.NoError(t, err)
					listener.requireErrors(
						t,
						listenerEventTypeErrRequirements{
							runnerListenerMockEventTypeStopping: errorRequirements{
								"*": nil,
							},
							runnerListenerMockEventTypeDone: errorRequirements{
								"*": nil,
							},
							runnerListenerMockEventTypeShutdown: errorRequirements{
								"*": nil,
							},
						},
					)
					recs.requireDoneFullLc(t, listener, nil)
				})
			})
		}
	})

	t.Run("wait returns err during starting", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					recs.setStartErr(nil, time.Second*100)
					onReady := func() {
						require.Fail(t, "onReady should not be called")
					}
					recs.triggerWaitErrOnClose(context.Canceled, time.Second*100)
					go func() {
						time.Sleep(250 * time.Second)
						// started: 0,1  2
						// in progress: 3
						recs[:3].requireStarted(t, listener, startOrderReq[:3], 2)
						recs[3].waiterMock.errChan <- testCause
					}()
					err := runner.Run(ctx, onReady)
					stopOrderReq := startOrderReq.Remove(3).Inverse()
					recs.requireDoneOrNotStarted(t, listener, stopOrderReq)
					componentFailedReq := func(err error, msgAndArgs []any) {
						var componentFailedErr ErrLifecycleHookFailed
						require.ErrorAs(t, err, &componentFailedErr, msgAndArgs...)
						require.Equal(t, any(3), componentFailedErr.LifecycleHook().Tag(), msgAndArgs...)
						require.ErrorIs(t, componentFailedErr, testCause, msgAndArgs...)
					}
					componentFailedReq(err, nil)
					listener.requireErrors(
						t,
						listenerEventTypeErrRequirements{
							runnerListenerMockEventTypeStopping: errorRequirements{
								"*": componentFailedReq, 3: testCause,
							},
							runnerListenerMockEventTypeDone: errorRequirements{
								"*": context.Canceled, 3: testCause,
							},
							runnerListenerMockEventTypeShutdown: errorRequirements{
								"*": componentFailedReq,
							},
						},
					)
				})
			})
		}
	})

	t.Run("stop all during starts", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant)
					ctx, cancel := context.WithCancel(context.Background())
					recs[:3].setStartErr(nil, time.Second*100)
					onReady := func() {
						require.Fail(t, "onReady should not be called")
					}
					go func() {
						time.Sleep(150 * time.Second)
						recs[:3].requireStarted(t, listener, startOrderReq[:3], 2)
						recs[3:].setStartErr(context.Canceled)
						recs[:3].setWaitErr(context.Canceled)
						recs.setCloseErr(context.Canceled)
						cancel()
					}()
					err := runner.Run(ctx, onReady)
					require.ErrorIs(t, err, context.Canceled)
				})
			})
		}
	})

	t.Run("wait returns nil on ready with OptNilRunResultAsError()", func(t *testing.T) {
		t.Parallel()
		for _, defsVariant := range defsVariants {
			t.Run(fmt.Sprintf("variant %v", defsVariant), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, startOrderReq, listener := createDefs(defsVariant, OptNilRunResultAsError())
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					recs.setStartErr(nil)
					recs.triggerWaitErrOnClose(context.Canceled, time.Second)
					isOnReadyCalled := false
					onReady := func() {
						isOnReadyCalled = true
						time.Sleep(time.Second)
						recs.requireStarted(t, listener, startOrderReq)
						recs[3].waiterMock.errChan <- nil
						go func() {
							time.Sleep(20 * time.Second)
							recs.requireDoneFullLc(t, listener,
								startOrderReq.Inverse().Remove(3),
							)
						}()
					}
					err := runner.Run(ctx, onReady)
					require.True(t, isOnReadyCalled)
					var lifecycleHookFailedErr ErrLifecycleHookFailed
					require.ErrorAs(t, err, &lifecycleHookFailedErr)
					require.Equal(t, lifecycleHookFailedErr.LifecycleHook().Tag(), 3)
					require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), ErrUnexpectedRunNilResult)
					isComponentFailedWithUnexpectedRunNilRunResultErr := func(err error, msgAndArgs []any) {
						var componentFailedErr ErrLifecycleHookFailed
						require.ErrorAs(t, err, &componentFailedErr, msgAndArgs...)
						require.Equal(t, any(3), componentFailedErr.LifecycleHook().Tag(), msgAndArgs...)
						require.ErrorIs(t, componentFailedErr.Unwrap(), ErrUnexpectedRunNilResult, msgAndArgs...)
					}
					listener.requireErrors(
						t,
						listenerEventTypeErrRequirements{
							runnerListenerMockEventTypeStopping: errorRequirements{
								3:   nil,
								"*": isComponentFailedWithUnexpectedRunNilRunResultErr,
							},
							runnerListenerMockEventTypeDone: errorRequirements{
								3:   ErrUnexpectedRunNilResult,
								"*": context.Canceled,
							},
							runnerListenerMockEventTypeShutdown: errorRequirements{
								"*": isComponentFailedWithUnexpectedRunNilRunResultErr,
							},
						},
					)
				})
			})
		}
	})
}

func TestRunner_RunPartialPhases(t *testing.T) {
	t.Cleanup(testsCleanup)

	rootsIndexesSet := [][]int{
		{3},
		{3, 2},
		{3, 2, 1},
		{3, 2, 1, 0},
		{3, 2, 1, 0, 4},
		{3, 4, 0},
	}

	createDefs := func(rootIndexes []int) (
		recs testDefRecs,
		r Runner,
		listener *runnerListenerMock,
	) {
		rec1runnableMock := newRunnableMock(1)
		recs = testDefRecs{
			{
				i:           0,
				starterMock: newStarterMock(0),
			},
			{
				i:           1,
				runnable:    rec1runnableMock,
				starterMock: rec1runnableMock.starterMock,
				waiterMock:  rec1runnableMock.waiterMock,
				closerMock:  rec1runnableMock.closerMock,
			},
			{
				i:          2,
				closerMock: newCloserMock(2),
			},
			{
				i:           3,
				starterMock: newStarterMock(3),
				closerMock:  newCloserMock(3),
			},
			{
				i:          4,
				closerMock: newCloserMock(4),
			},
		}
		recs[0].getComponent = Provide(func() int {
			UseLifecycle().
				AddStarter(recs[0].starterMock).
				Tag(recs[0].starterMock.tag)
			UseTag("zero")
			return 0
		})
		recs[1].getComponent = Provide(func() int {
			UseLifecycle().
				AddRunnable(recs[1].runnable).
				Tag(recs[1].starterMock.tag)
			recs[0].getComponent()
			recs[4].getComponent()
			return 1
		})
		recs[2].getComponent = Provide(func() int {
			UseLifecycle().
				AddCloser(recs[2].closerMock).
				Tag(recs[2].closerMock.tag)
			recs[1].getComponent()
			return 2
		})
		recs[3].getComponent = Provide(func() int {
			UseLifecycle().
				AddStarter(recs[3].starterMock).
				AddCloser(recs[3].closerMock).
				Tag(recs[3].starterMock.tag)
			recs[2].getComponent()
			return 2
		})
		recs[4].getComponent = Provide(func() int {
			UseLifecycle().
				AddCloser(recs[4].closerMock).
				Tag(recs[4].closerMock.tag)
			return 4
		})

		listener = &runnerListenerMock{}
		listener.ShouldBeCalledInCurrentGoroutineId()
		r, _ = NewRunnerE(func() error {
			for _, rootIndex := range rootIndexes {
				recs[rootIndex].getComponent()
			}
			return nil
		}, OptRunnerListeners(listener))
		return recs, r, listener
	}

	t.Run("cancel before ready", func(t *testing.T) {
		t.Parallel()
		for _, rootsIndexes := range rootsIndexesSet {
			t.Run(fmt.Sprintf("rootsIndexes %v", rootsIndexes), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, listener := createDefs(rootsIndexes)
					recs[1:2].triggerWaitErrOnClose(context.Canceled)
					_ = listener
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					var isRunDone atomic.Bool
					go func() {
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.enterEventId.Load())
						listener.requireTagEvent(t, runnerListenerMockEventTypeStart, 0, func(event runnerListenerMockEvent) {
							require.Equal(t, "zero", event.info.ComponentInfo().Tag())
						})
						require.Negative(t, recs[0].starterMock.exitEventId.Load())
						require.Negative(t, recs[1].waiterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.enterEventId.Load())
						recs[0].starterMock.errChan <- nil
						recs[1].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[3].starterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.exitEventId.Load())
						cancel()
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.exitEventId.Load())
						require.ErrorIs(t, *recs[3].starterMock.ctxDoneCause.Load(), context.Canceled)
						require.Negative(t, recs[3].closerMock.enterEventId.Load())
						require.Positive(t, recs[2].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.exitEventId.Load())
						recs[2].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[2].closerMock.exitEventId.Load())
						require.Positive(t, recs[1].waiterMock.enterEventId.Load())
						require.Positive(t, recs[1].waiterMock.exitEventId.Load())
						require.False(t, isRunDone.Load())
						recs[1].waiterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[1].waiterMock.exitEventId.Load())
						require.Positive(t, recs[4].closerMock.enterEventId.Load())
						require.False(t, isRunDone.Load())
						recs[4].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[4].closerMock.exitEventId.Load())
						require.True(t, isRunDone.Load())
					}()
					err := runner.Run(ctx, func() {
						require.Fail(t, "onReady should not be called")
					})
					isRunDone.Store(true)
					require.ErrorIs(t, err, ctx.Err())
				})
			})
		}
	})

	t.Run("cancel after ready", func(t *testing.T) {
		t.Parallel()
		for _, rootIndexes := range rootsIndexesSet {
			t.Run(fmt.Sprintf("rootIndexes %v", rootIndexes), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, listener := createDefs(rootIndexes)
					_ = listener
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					var isOnReadyCalled atomic.Bool
					var isRunDone atomic.Bool
					go func() {
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.enterEventId.Load())
						recs[0].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.exitEventId.Load())
						recs[1].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[1].waiterMock.enterEventId.Load())
						require.Positive(t, recs[3].starterMock.enterEventId.Load())
						require.False(t, isOnReadyCalled.Load())
						recs[3].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[3].starterMock.exitEventId.Load())
						require.True(t, isOnReadyCalled.Load())
					}()

					err := runner.Run(ctx, func() {
						isOnReadyCalled.Store(true)
						cancel()
						go func() {
							time.Sleep(time.Second)
							require.Positive(t, recs[3].closerMock.enterEventId.Load())
							require.Negative(t, recs[3].closerMock.exitEventId.Load())
							require.Negative(t, recs[2].closerMock.enterEventId.Load())
							recs[3].closerMock.errChan <- nil
							time.Sleep(time.Second)
							require.Positive(t, recs[3].closerMock.exitEventId.Load())
							require.Positive(t, recs[2].closerMock.enterEventId.Load())
							require.Negative(t, recs[2].closerMock.exitEventId.Load())
							recs[2].closerMock.errChan <- nil
							time.Sleep(time.Second)
							require.Positive(t, recs[2].closerMock.exitEventId.Load())
							require.False(t, isRunDone.Load())
							recs[1].closerMock.errChan <- nil
							recs[1].waiterMock.errChan <- nil
							time.Sleep(time.Second)
							require.Positive(t, recs[1].waiterMock.exitEventId.Load())
							require.False(t, isRunDone.Load())
							recs[4].closerMock.errChan <- nil
							time.Sleep(time.Second)
							require.Positive(t, recs[4].closerMock.exitEventId.Load())
							require.True(t, isRunDone.Load())
						}()
					})
					isRunDone.Store(true)
					require.True(t, isOnReadyCalled.Load())
					require.ErrorIs(t, err, ctx.Err())
				})
			})
		}
	})

	t.Run("start err", func(t *testing.T) {
		t.Parallel()
		for _, rootIndexes := range rootsIndexesSet {
			t.Run(fmt.Sprintf("rootIndexes %v", rootIndexes), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, listener := createDefs(rootIndexes)
					_ = listener
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					startErr := errors.New("start_err")
					var isRunDone atomic.Bool
					go func() {
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.enterEventId.Load())
						require.Negative(t, recs[0].starterMock.exitEventId.Load())
						require.Negative(t, recs[1].waiterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.enterEventId.Load())
						recs[0].starterMock.errChan <- startErr
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.exitEventId.Load())
						require.Negative(t, recs[3].starterMock.enterEventId.Load())
						require.Negative(t, recs[3].closerMock.enterEventId.Load())
						require.Positive(t, recs[2].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.exitEventId.Load())
						recs[2].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[2].closerMock.exitEventId.Load())
						require.Negative(t, recs[1].waiterMock.enterEventId.Load())
						require.Negative(t, recs[1].starterMock.enterEventId.Load())
						require.False(t, isRunDone.Load())
						recs[4].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[4].closerMock.exitEventId.Load())
						require.True(t, isRunDone.Load())
					}()
					err := runner.Run(ctx, func() {
						require.Fail(t, "onReady should not be called")
					})
					isRunDone.Store(true)
					require.NoError(t, ctx.Err())
					var failedErr ErrLifecycleHookFailed
					require.ErrorIs(t, err, startErr)
					require.ErrorAs(t, err, &failedErr)
					require.Equal(t, any(0), failedErr.LifecycleHook().Tag())
				})
			})
		}
	})

	t.Run("wait returns nil", func(t *testing.T) {
		t.Parallel()
		for _, rootIndexes := range rootsIndexesSet {
			t.Run(fmt.Sprintf("rootIndexes %v", rootIndexes), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, listener := createDefs(rootIndexes)
					_ = listener
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					var isRunDone atomic.Bool
					go func() {
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.enterEventId.Load())
						require.Negative(t, recs[0].starterMock.exitEventId.Load())
						require.Negative(t, recs[1].waiterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.enterEventId.Load())
						recs[0].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.exitEventId.Load())
						require.Positive(t, recs[1].starterMock.enterEventId.Load())
						recs[1].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[3].starterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.exitEventId.Load())
						recs[3].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[3].starterMock.exitEventId.Load())
						require.Negative(t, recs[3].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.enterEventId.Load())
						recs[1].waiterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[1].waiterMock.exitEventId.Load())
						require.Negative(t, recs[3].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.enterEventId.Load())
						require.Negative(t, recs[4].closerMock.enterEventId.Load())

						cancel()
						time.Sleep(time.Second)
						require.Positive(t, recs[3].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.enterEventId.Load())
						recs[3].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[3].closerMock.exitEventId.Load())
						require.Positive(t, recs[2].closerMock.enterEventId.Load())
						require.False(t, isRunDone.Load())
						recs[2].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[2].closerMock.exitEventId.Load())
						require.False(t, isRunDone.Load())
						recs[4].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[4].closerMock.exitEventId.Load())
						require.True(t, isRunDone.Load())
					}()
					var isOnReadyCalled atomic.Bool
					err := runner.Run(ctx, func() {
						isOnReadyCalled.Store(true)
						require.Positive(t, recs[0].starterMock.exitEventId.Load())
						require.Positive(t, recs[3].starterMock.exitEventId.Load())
					})
					isRunDone.Store(true)
					require.True(t, isOnReadyCalled.Load())
					require.ErrorIs(t, err, ctx.Err())
				})
			})
		}
	})

	t.Run("wait returns err after ready", func(t *testing.T) {
		t.Parallel()
		for _, rootIndexes := range rootsIndexesSet {
			t.Run(fmt.Sprintf("rootIndexes %v", rootIndexes), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, listener := createDefs(rootIndexes)
					_ = listener
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					waitErr := errors.New("wait_err")
					var isRunDone atomic.Bool
					var isOnReadyCalled atomic.Bool

					go func() {
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.enterEventId.Load())
						require.Negative(t, recs[0].starterMock.exitEventId.Load())
						require.Negative(t, recs[1].waiterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.enterEventId.Load())
						recs[0].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.exitEventId.Load())
						require.Positive(t, recs[1].starterMock.enterEventId.Load())
						recs[1].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[3].starterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.exitEventId.Load())
						require.False(t, isOnReadyCalled.Load())
						recs[3].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.True(t, isOnReadyCalled.Load())
						require.Positive(t, recs[3].starterMock.exitEventId.Load())
						require.Negative(t, recs[3].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.enterEventId.Load())
						recs[1].waiterMock.errChan <- waitErr
						time.Sleep(time.Second)
						require.Positive(t, recs[1].waiterMock.exitEventId.Load())
						require.Positive(t, recs[3].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.enterEventId.Load())
						recs[3].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[3].closerMock.exitEventId.Load())
						require.Positive(t, recs[2].closerMock.enterEventId.Load())
						require.False(t, isRunDone.Load())
						recs[2].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[2].closerMock.exitEventId.Load())
						require.False(t, isRunDone.Load())
						recs[4].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[4].closerMock.exitEventId.Load())
						require.True(t, isRunDone.Load())
					}()

					err := runner.Run(ctx, func() {
						isOnReadyCalled.Store(true)
						require.Positive(t, recs[0].starterMock.exitEventId.Load())
						require.Positive(t, recs[3].starterMock.exitEventId.Load())
					})
					isRunDone.Store(true)
					require.True(t, isOnReadyCalled.Load())
					require.NoError(t, ctx.Err())
					var failedErr ErrLifecycleHookFailed
					require.ErrorIs(t, err, waitErr)
					require.ErrorAs(t, err, &failedErr)
					require.Equal(t, any(1), failedErr.LifecycleHook().Tag())
				})
			})
		}
	})

	t.Run("wait returns err during starting", func(t *testing.T) {
		t.Parallel()
		for _, rootIndexes := range rootsIndexesSet {
			t.Run(fmt.Sprintf("rootIndexes %v", rootIndexes), func(t *testing.T) {
				t.Parallel()
				synctest.Run(func() {
					recs, runner, listener := createDefs(rootIndexes)
					recs[3].starterMock.doNotExitOnCtxDone = true
					_ = listener
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					waitErr := errors.New("wait_err")
					var isRunDone atomic.Bool

					go func() {
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.enterEventId.Load())
						require.Negative(t, recs[0].starterMock.exitEventId.Load())
						require.Negative(t, recs[1].waiterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.enterEventId.Load())
						recs[0].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[0].starterMock.exitEventId.Load())
						require.Positive(t, recs[1].starterMock.enterEventId.Load())
						recs[1].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[1].waiterMock.enterEventId.Load())
						require.Positive(t, recs[3].starterMock.enterEventId.Load())
						require.Negative(t, recs[3].starterMock.exitEventId.Load())
						recs[1].waiterMock.errChan <- waitErr
						time.Sleep(time.Second)
						require.Positive(t, recs[1].waiterMock.exitEventId.Load())
						require.Negative(t, recs[3].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.enterEventId.Load())
						require.Positive(t, recs[4].closerMock.enterEventId.Load())
						recs[3].starterMock.errChan <- nil
						time.Sleep(time.Second)
						require.False(t, isRunDone.Load())
						require.Positive(t, recs[3].starterMock.exitEventId.Load())
						require.Positive(t, recs[3].closerMock.enterEventId.Load())
						require.Negative(t, recs[2].closerMock.enterEventId.Load())
						recs[3].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[3].closerMock.exitEventId.Load())
						require.Positive(t, recs[2].closerMock.enterEventId.Load())
						recs[2].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[2].closerMock.exitEventId.Load())
						require.Positive(t, recs[4].closerMock.enterEventId.Load())
						require.False(t, isRunDone.Load())
						recs[4].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.Positive(t, recs[4].closerMock.exitEventId.Load())
						require.True(t, isRunDone.Load())
					}()

					err := runner.Run(ctx, func() {
						require.Fail(t, "onReady should not be called")
					})
					isRunDone.Store(true)
					require.NoError(t, ctx.Err())
					var failedErr ErrLifecycleHookFailed
					require.ErrorIs(t, err, waitErr)
					require.ErrorAs(t, err, &failedErr)
					require.Equal(t, any(1), failedErr.LifecycleHook().Tag())
				})
			})
		}
	})
}

func TestRunner_RunWithNilContext(t *testing.T) {
	t.Cleanup(testsCleanup)
	synctest.Run(func() {

		runnable := newReadinessRunnableMock(1)
		def1 := Provide(func() int {
			UseLifecycle().AddReadinessRunnable(runnable).Tag(1)
			return 1
		})
		listener := &runnerListenerMock{}
		listener.ShouldBeCalledInCurrentGoroutineId()
		r, _ := NewRunnerE(func() error {
			def1()
			return nil
		}, OptRunnerListeners(listener))
		var isOnReadyCalled atomic.Bool
		var isRunDone atomic.Bool
		go func() {
			time.Sleep(time.Second)
			require.Equal(t, 1, signals.MockSubscribersCount())
			require.False(t, isOnReadyCalled.Load())
			runnable.starterMock.errChan <- nil
			time.Sleep(time.Second)
			require.True(t, isOnReadyCalled.Load())
			signals.SendMockSignal(syscall.SIGINT)
			time.Sleep(time.Second)
			require.Equal(t, 0, signals.MockSubscribersCount())
			require.False(t, isRunDone.Load())
			runnable.closerMock.errChan <- nil
			time.Sleep(time.Second)
			require.True(t, isRunDone.Load())
		}()
		err := r.Run(nil, func() {
			isOnReadyCalled.Store(true)
			require.Equal(t, map[any]int{1: 0}, listener.getEventsOrderMap(runnerListenerMockEventTypeReady))
		})
		isRunDone.Store(true)
		require.True(t, isOnReadyCalled.Load())
		require.ErrorIs(t, err, context.Canceled)
		waitCtxDoneCause := *runnable.waiterMock.ctxDoneCause.Load()
		var signalErr contexts.ErrSignalReceived
		require.ErrorAs(t, waitCtxDoneCause, &signalErr)
		require.Equal(t, syscall.SIGINT, signalErr.Signal)
	})
}

func TestRunner_OptStartTimeout(t *testing.T) {
	createDefs := func(
		def0StartTimeout time.Duration,
		def1StartTimeout time.Duration,
		runnerStartTimeout time.Duration,
	) (
		recs testDefRecs,
		r Runner,
	) {
		rec1readinessRunnableMock := newReadinessRunnableMock(1)
		recs = testDefRecs{
			{
				i:           0,
				starterMock: newStarterMock(0),
			},
			{
				i:                 1,
				readinessRunnable: rec1readinessRunnableMock,
				starterMock:       rec1readinessRunnableMock.starterMock,
				waiterMock:        rec1readinessRunnableMock.waiterMock,
				closerMock:        rec1readinessRunnableMock.closerMock,
			},
		}
		recs[0].getComponent = Provide(func() int {
			var options []StarterOption
			if def0StartTimeout > 0 {
				options = append(options, OptStartTimeout(def0StartTimeout))
			}
			UseLifecycle().
				AddStarter(recs[0].starterMock, options...).
				Tag(recs[0].starterMock.tag)
			return 0
		})
		recs[1].getComponent = Provide(func() int {
			var options []ReadinessRunnableOption
			if def1StartTimeout > 0 {
				options = append(options, OptStartTimeout(def1StartTimeout))
			}
			UseLifecycle().
				AddReadinessRunnable(recs[1].readinessRunnable, options...).
				Tag(recs[1].starterMock.tag)
			return 1
		})
		var runnerOptions []RunnerOption
		if runnerStartTimeout > 0 {
			runnerOptions = append(runnerOptions, OptStartTimeout(runnerStartTimeout))
		}
		r = NewRunner(func() {
			recs[0].getComponent()
			recs[1].getComponent()
		}, runnerOptions...)
		return recs, r
	}

	t.Run("global OptStartTimeout", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			recs, runner := createDefs(0, 0, time.Second)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			recs.setStartErr(nil, time.Second*2)
			recs.triggerWaitErrOnClose(context.Canceled)
			onReady := func() {
				require.Fail(t, "onReady should not be called")
			}
			err := runner.Run(ctx, onReady)
			require.True(
				t,
				errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded),
				"err: %v",
				err,
			)
		})
	})

	t.Run("def0 OptStartTimeout", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			recs, runner := createDefs(time.Second, 0, 0)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			recs.setStartErr(nil, time.Second*2)
			recs.triggerWaitErrOnClose(context.Canceled)
			onReady := func() {
				require.Fail(t, "onReady should not be called")
			}
			err := runner.Run(ctx, onReady)
			var lifecycleHookFailedErr ErrLifecycleHookFailed
			require.ErrorAs(t, err, &lifecycleHookFailedErr)
			require.Equal(t, any(0), lifecycleHookFailedErr.LifecycleHook().Tag())
			require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), context.DeadlineExceeded)

			rec0Err := *recs[0].starterMock.ctxDoneCause.Load()
			require.ErrorIs(t, rec0Err, context.DeadlineExceeded)

			rec1err := *recs[1].starterMock.ctxDoneCause.Load()
			require.ErrorAs(t, rec1err, &lifecycleHookFailedErr)
			require.Equal(t, any(0), lifecycleHookFailedErr.LifecycleHook().Tag())
			require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), context.DeadlineExceeded)
		})
	})

	t.Run("def1 OptStartTimeout", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			recs, runner := createDefs(0, time.Second, 0)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			recs.setStartErr(nil, time.Second*2)
			recs.triggerWaitErrOnClose(context.Canceled)
			onReady := func() {
				require.Fail(t, "onReady should not be called")
			}
			err := runner.Run(ctx, onReady)
			var lifecycleHookFailedErr ErrLifecycleHookFailed
			require.ErrorAs(t, err, &lifecycleHookFailedErr)
			require.Equal(t, any(1), lifecycleHookFailedErr.LifecycleHook().Tag())
			require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), context.Canceled)

			rec0Err := *recs[0].starterMock.ctxDoneCause.Load()
			require.ErrorAs(t, rec0Err, &lifecycleHookFailedErr)
			require.Equal(t, any(1), lifecycleHookFailedErr.LifecycleHook().Tag())
			require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), context.Canceled)

			rec1Err := *recs[1].starterMock.ctxDoneCause.Load()
			require.ErrorIs(t, rec1Err, context.DeadlineExceeded)
		})
	})

	t.Run("def0 and runner OptStartTimeout", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			recs, runner := createDefs(time.Second*2, time.Second*3, time.Second)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			recs.setStartErr(nil, time.Second*4)
			recs.triggerWaitErrOnClose(context.Canceled)
			onReady := func() {
				require.Fail(t, "onReady should not be called")
			}
			var isRunnerDone atomic.Bool
			go func() {
				time.Sleep(1100 * time.Millisecond)
				require.False(t, isRunnerDone.Load())
				time.Sleep(time.Second)
				require.True(t, isRunnerDone.Load())
			}()
			err := runner.Run(ctx, onReady)
			isRunnerDone.Store(true)
			var lifecycleHookFailedErr ErrLifecycleHookFailed
			require.ErrorAs(t, err, &lifecycleHookFailedErr)
			require.Equal(t, any(0), lifecycleHookFailedErr.LifecycleHook().Tag())
			require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), context.DeadlineExceeded)

			rec0Err := *recs[0].starterMock.ctxDoneCause.Load()
			require.ErrorIs(t, rec0Err, context.DeadlineExceeded)

			rec1err := *recs[1].starterMock.ctxDoneCause.Load()
			require.ErrorAs(t, rec1err, &lifecycleHookFailedErr)
			require.Equal(t, any(0), lifecycleHookFailedErr.LifecycleHook().Tag())
			require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), context.DeadlineExceeded)
		})
	})
}

func TestRunner_OptNilRunResultAsError(t *testing.T) {
	createDefs := func(
		isDef0NilRunResultAsError bool,
		isDef1NilRunResultAsError bool,
		isRunnerNilRunResultAsError bool,
	) (
		recs testDefRecs,
		r Runner,
	) {
		rec0RunnableMock := newRunnableMock(0)
		rec1readinessRunnableMock := newReadinessRunnableMock(1)
		recs = testDefRecs{
			{
				i:           0,
				runnable:    rec0RunnableMock,
				starterMock: rec0RunnableMock.starterMock,
				waiterMock:  rec0RunnableMock.waiterMock,
				closerMock:  rec0RunnableMock.closerMock,
			},
			{
				i:                 1,
				readinessRunnable: rec1readinessRunnableMock,
				starterMock:       rec1readinessRunnableMock.starterMock,
				waiterMock:        rec1readinessRunnableMock.waiterMock,
				closerMock:        rec1readinessRunnableMock.closerMock,
			},
		}
		recs[0].getComponent = Provide(func() int {
			var options []RunnableOption
			if isDef0NilRunResultAsError {
				options = append(options, OptNilRunResultAsError())
			}
			UseLifecycle().
				AddRunnable(recs[0].runnable, options...).
				Tag(recs[0].starterMock.tag)
			return 0
		})
		recs[1].getComponent = Provide(func() int {
			var options []ReadinessRunnableOption
			if isDef1NilRunResultAsError {
				options = append(options, OptNilRunResultAsError())
			}
			UseLifecycle().
				AddReadinessRunnable(recs[1].readinessRunnable, options...).
				Tag(recs[1].starterMock.tag)
			return 1
		})
		var runnerOptions []RunnerOption
		if isRunnerNilRunResultAsError {
			runnerOptions = append(runnerOptions, OptNilRunResultAsError())
		}
		r = NewRunner(func() {
			recs[0].getComponent()
			recs[1].getComponent()
		}, runnerOptions...)
		return recs, r
	}

	t.Run("global OptNilRunResultAsError", func(t *testing.T) {
		t.Cleanup(testsCleanup)
		synctest.Run(func() {
			recs, runner := createDefs(false, false, true)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			recs.setStartErr(nil, time.Second)
			var isOnReadyCalled atomic.Bool
			var isRunnerDone atomic.Bool
			onReady := func() {
				isOnReadyCalled.Store(true)
				go func() {
					recs[0].waiterMock.errChan <- nil
					time.Sleep(time.Second)
					recs[1].closerMock.errChan <- nil
					time.Sleep(time.Second)
					require.True(t, isRunnerDone.Load())
				}()
			}
			err := runner.Run(ctx, onReady)
			require.True(t, isOnReadyCalled.Load())
			isRunnerDone.Store(true)
			var lifecycleHookFailedErr ErrLifecycleHookFailed
			require.ErrorAs(t, err, &lifecycleHookFailedErr)
			require.Equal(t, any(0), lifecycleHookFailedErr.LifecycleHook().Tag())
			require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), ErrUnexpectedRunNilResult)
			require.ErrorAs(t, *recs[1].waiterMock.ctxDoneCause.Load(), &lifecycleHookFailedErr)
			require.Equal(t, any(0), lifecycleHookFailedErr.LifecycleHook().Tag())
			require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), ErrUnexpectedRunNilResult)
		})
	})

	t.Run("def0 OptNilRunResultAsError", func(t *testing.T) {
		t.Run("def0 returns nil", func(t *testing.T) {
			t.Cleanup(testsCleanup)
			synctest.Run(func() {
				recs, runner := createDefs(true, false, false)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				recs.setStartErr(nil, time.Second)
				var isOnReadyCalled atomic.Bool
				var isRunnerDone atomic.Bool
				onReady := func() {
					isOnReadyCalled.Store(true)
					go func() {
						recs[0].waiterMock.errChan <- nil
						time.Sleep(time.Second)
						recs[1].closerMock.errChan <- nil
						time.Sleep(time.Second)
						require.True(t, isRunnerDone.Load())
					}()
				}
				err := runner.Run(ctx, onReady)
				require.True(t, isOnReadyCalled.Load())
				isRunnerDone.Store(true)
				var lifecycleHookFailedErr ErrLifecycleHookFailed
				require.ErrorAs(t, err, &lifecycleHookFailedErr)
				require.Equal(t, any(0), lifecycleHookFailedErr.LifecycleHook().Tag())
				require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), ErrUnexpectedRunNilResult)
				require.ErrorAs(t, *recs[1].waiterMock.ctxDoneCause.Load(), &lifecycleHookFailedErr)
				require.Equal(t, any(0), lifecycleHookFailedErr.LifecycleHook().Tag())
				require.ErrorIs(t, lifecycleHookFailedErr.Unwrap(), ErrUnexpectedRunNilResult)
			})
		})

		t.Run("def1 returns nil", func(t *testing.T) {
			t.Cleanup(testsCleanup)
			synctest.Run(func() {
				recs, runner := createDefs(true, false, false)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				recs.setStartErr(nil, time.Second)
				var isOnReadyCalled atomic.Bool
				var isRunnerDone atomic.Bool
				onReady := func() {
					isOnReadyCalled.Store(true)
					go func() {
						recs[1].waiterMock.errChan <- nil
						time.Sleep(time.Second)
						recs[0:1].triggerWaitErrOnClose(context.Canceled)
						time.Sleep(time.Second)
						require.False(t, isRunnerDone.Load())
						cancel()
						time.Sleep(time.Second)
						require.True(t, isRunnerDone.Load())
					}()
				}
				err := runner.Run(ctx, onReady)
				isRunnerDone.Store(true)
				require.ErrorIs(t, err, ctx.Err())
			})
		})
	})
}

func TestRunner_Run_SingleStarter(t *testing.T) {
	testErr := errors.New("test error")
	a := Provide(func() int {
		UseTag("tagA")
		UseLifecycle().AddStartFn(func(ctx context.Context) error {
			return testErr
		}).Tag("lcA")
		return 1
	})

	runner := NewRunner(func() {
		a()
	})
	runnerErr := runner.Run(context.Background(), nil)
	var lifecycleHookFailedErr ErrLifecycleHookFailed
	require.ErrorAs(t, runnerErr, &lifecycleHookFailedErr)
	require.Equal(t, "tagA", lifecycleHookFailedErr.LifecycleHook().ComponentInfo().Tag())
	require.Equal(t, "lcA", lifecycleHookFailedErr.LifecycleHook().Tag())
	require.Equal(t, testErr, lifecycleHookFailedErr.Unwrap())
}
