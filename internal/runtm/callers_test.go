package runtm

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindClosestKnownCallerCtxInCallStack(t *testing.T) {
	nextLineCallerCtx := func() CallerCtx {
		cctx := NewCallerCtx(1)
		cctx.line++
		return cctx
	}

	t.Run("detects", func(t *testing.T) {
		for i := 0; i <= 9; i++ {
			t.Run(fmt.Sprintf("receive_in_%d", i), func(t *testing.T) {
				t.Parallel()

				var f1ctx CallerCtx
				var f3ctx CallerCtx
				var externalCtx CallerCtx
				func() {
					externalCtx = NewCallerCtx(0)
				}()

				receiver := func() {
					_, found, isFound := FindClosestKnownCallerCtxInCallStack(1, f1ctx, f3ctx)
					require.True(t, isFound)
					if !f3ctx.Empty() {
						require.Equal(t, f3ctx, found)
					} else {
						require.Equal(t, f1ctx, found)
					}

					_, found, isFound = FindClosestKnownCallerCtxInCallStack(1, f3ctx, f1ctx)
					require.True(t, isFound)
					if !f3ctx.Empty() {
						require.Equal(t, f3ctx, found)
					} else {
						require.Equal(t, f1ctx, found)
					}

					_, found, isFound = FindClosestKnownCallerCtxInCallStack(1, f3ctx)
					if !f3ctx.Empty() {
						require.True(t, isFound)
						require.Equal(t, f3ctx, found)
					} else {
						require.False(t, isFound)
					}

					_, found, isFound = FindClosestKnownCallerCtxInCallStack(1, f1ctx)
					require.True(t, isFound)
					require.Equal(t, f1ctx, found)

					_, found, isFound = FindClosestKnownCallerCtxInCallStack(1, externalCtx)
					require.False(t, isFound)

					_, found, isFound = FindClosestKnownCallerCtxInCallStack(1)
					require.False(t, isFound)
				}
				f10 := func(counter int) {
					receiver()
				}
				f9 := func(counter int) {
					if counter == 0 {
						receiver()
					} else {
						f10(counter - 1)
					}
				}
				f8 := func(counter int) {
					if counter == 0 {
						receiver()
					} else {
						f9(counter - 1)
					}
				}
				f7 := func(counter int) {
					if counter == 0 {
						receiver()
					} else {
						f8(counter - 1)
					}
				}
				f6 := func(counter int) {
					if counter == 0 {
						receiver()
					} else {
						f7(counter - 1)
					}
				}
				f5 := func(counter int) {
					if counter == 0 {
						receiver()
					} else {
						f6(counter - 1)
					}
				}
				f4 := func(counter int) {
					if counter == 0 {
						receiver()
					} else {
						f5(counter - 1)
					}
				}
				f3 := func(counter int) {
					f3ctx = NewCallerCtx(0)
					if counter == 0 {
						receiver()
					} else {
						f4(counter - 1)
					}
				}
				f2 := func(counter int) {
					if counter == 0 {
						receiver()
					} else {
						f3(counter - 1)
					}
				}
				f1 := func(counter int) {
					f1ctx = NewCallerCtx(0)
					if counter == 0 {
						receiver()
					} else {
						f2(counter - 1)
					}
				}
				f1(i)
			})
		}
	})

	t.Run("returns userCallerCtxs", func(t *testing.T) {
		var f2ctx CallerCtx
		var f3ctx CallerCtx
		var f4ctx CallerCtx

		receiver := func() bool {
			userCallerCtxs, _, isInCallStack := FindClosestKnownCallerCtxInCallStack(2, f4ctx)
			require.True(t, isInCallStack)
			require.Equal(t, CallerCtxs{f2ctx, f3ctx}, userCallerCtxs)
			return true
		}
		f1 := func() bool {
			receiver()
			return false
		}
		f2 := func() bool {
			// create CallerCtx at the same line as f1() call
			if f2ctx = NewCallerCtx(0); f1() {
				return true
			}
			return false
		}
		f3 := func() bool {
			if f3ctx = NewCallerCtx(0); f2() {
				return true
			}
			return false
		}
		f4 := func() bool {
			if f4ctx = NewCallerCtx(0); f3() {
				return true
			}
			return false
		}
		f4()
	})

	t.Run("traces panic", func(t *testing.T) {
		var p1CallerCtx CallerCtx
		f1 := func() {
			p1CallerCtx = nextLineCallerCtx()
			panic("p1")
		}
		var f2CallerCtx CallerCtx
		f2 := func() {
			f2CallerCtx = nextLineCallerCtx()
			f1()
		}
		var f3Boundary CallerCtx
		f3 := func() {
			if f3Boundary.Empty() {
				f3Boundary = NewCallerCtx(0)
			} else {
				f2()
			}
		}
		f3()
		require.False(t, f3Boundary.Empty())
		func() {
			defer func() {
				rec := recover()
				require.Equal(t, "p1", rec)
				userCallerCtxs, foundCallerCtx, isFound := FindClosestKnownCallerCtxInCallStack(
					PanicRecoveryDeferFnCallerLevel, f3Boundary,
				)
				require.True(t, isFound)
				require.Equal(t, f3Boundary, foundCallerCtx)
				require.Equal(t, CallerCtxs{p1CallerCtx, f2CallerCtx}, userCallerCtxs)
			}()
			f3()
		}()
	})
}

func TestGetGoroutineID(t *testing.T) {
	t.Parallel()
	gid := GetGoroutineID()
	gid2 := GetGoroutineID()
	require.NotEmpty(t, gid)
	require.Equal(t, gid, gid2)
	var wg sync.WaitGroup
	wg.Add(2)
	var gid3, gid4 string
	go func() {
		gid3 = GetGoroutineID()
		wg.Done()
	}()
	nested := func() {
		gid4 = GetGoroutineID()
	}
	go func() {
		nested()
		wg.Done()
	}()
	wg.Wait()
	require.NotEmpty(t, gid3)
	require.NotEmpty(t, gid4)
	require.NotEqual(t, gid, gid3)
	require.NotEqual(t, gid, gid4)
	require.NotEqual(t, gid3, gid4)
}
