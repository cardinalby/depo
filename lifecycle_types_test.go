package depo

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

var lastRunnableEventSeqNo atomic.Int64

type lcPhaseMock struct {
	errChan            chan error
	enterSleepDuration atomic.Int64
	exitSleepDuration  atomic.Int64
	enterEventId       atomic.Int64
	exitEventId        atomic.Int64
	ctxDoneCause       atomic.Pointer[error]
	phaseAction        func()
	doNotExitOnCtxDone bool
}

func newLcPhaseMock() *lcPhaseMock {
	pm := &lcPhaseMock{
		errChan: make(chan error, 1),
	}
	pm.enterEventId.Store(-1)
	pm.exitEventId.Store(-1)
	return pm
}

func (l *lcPhaseMock) exec(ctx context.Context) chan error {
	if old := l.enterEventId.Swap(lastRunnableEventSeqNo.Add(1)); old != -1 {
		panic(fmt.Sprintf("enterEventId already set: %d, new: %d", old, l.enterEventId.Load()))
	}
	if d := l.enterSleepDuration.Load(); d > 0 {
		time.Sleep(time.Duration(d))
	}
	if ctx == nil {
		ctx = context.Background()
	}

	res := make(chan error, 1)
	go func() {
		select {
		case err := <-l.errChan:
			if old := l.exitEventId.Swap(lastRunnableEventSeqNo.Add(1)); old != -1 {
				panic(fmt.Sprintf("exitEventId already set: %d, new: %d", old, l.exitEventId.Load()))
			}
			time.Sleep(time.Duration(l.exitSleepDuration.Load()))
			res <- err
		case <-ctx.Done():
			if old := l.exitEventId.Swap(lastRunnableEventSeqNo.Add(1)); old != -1 {
				panic(fmt.Sprintf("exitEventId already set: %d, new: %d", old, l.exitEventId.Load()))
			}
			time.Sleep(time.Duration(l.exitSleepDuration.Load()))
			cause := context.Cause(ctx)
			l.ctxDoneCause.Store(&cause)
			if l.doNotExitOnCtxDone {
				res <- <-l.errChan
			} else {
				res <- ctx.Err()
			}
		}
	}()
	if l.phaseAction != nil {
		l.phaseAction()
	}
	return res
}

type readinessRunnableMock struct {
	starterMock *starterMock
	waiterMock  *waiterMock
	closerMock  *closerMock
}

func newReadinessRunnableMock(tag int) *readinessRunnableMock {
	return &readinessRunnableMock{
		starterMock: newStarterMock(tag),
		waiterMock:  newWaiterMock(tag),
		closerMock:  newCloserMock(tag),
	}
}

func (mr *readinessRunnableMock) Run(ctx context.Context, onReady func()) error {
	if err := <-mr.starterMock.exec(ctx); err != nil {
		return err
	}
	onReady()

	internalCtx, cancelInternalCtx := context.WithCancelCause(context.Background())
	defer cancelInternalCtx(nil)
	waitRes := mr.waiterMock.exec(internalCtx)
	select {
	case err := <-waitRes:
		return err
	case <-ctx.Done():
		<-mr.closerMock.exec(internalCtx)
		cancelInternalCtx(context.Cause(ctx))
		return <-waitRes
	}
}

type runnableMock struct {
	starterMock *starterMock
	waiterMock  *waiterMock
	closerMock  *closerMock
}

func newRunnableMock(tag int) *runnableMock {
	return &runnableMock{
		starterMock: newStarterMock(tag),
		waiterMock:  newWaiterMock(tag),
		closerMock:  newCloserMock(tag),
	}
}

func (mr *runnableMock) Run(ctx context.Context) error {
	if err := <-mr.starterMock.exec(ctx); err != nil {
		return err
	}

	internalCtx, cancelInternalCtx := context.WithCancelCause(context.Background())
	defer cancelInternalCtx(nil)
	waitRes := mr.waiterMock.exec(internalCtx)
	select {
	case err := <-waitRes:
		return err
	case <-ctx.Done():
		<-mr.closerMock.exec(internalCtx)
		cancelInternalCtx(context.Cause(ctx))
		return <-waitRes
	}
}

type starterMock struct {
	*lcPhaseMock
	tag int
}

func newStarterMock(tag int) *starterMock {
	return &starterMock{
		tag:         tag,
		lcPhaseMock: newLcPhaseMock(),
	}
}

func (s *starterMock) Start(ctx context.Context) error {
	fmt.Printf("%s Starting component with tag: %d\n", time.Now().String(), s.tag)
	return <-s.exec(ctx)
}

type waiterMock struct {
	*lcPhaseMock
	tag int
}

func newWaiterMock(tag int) *waiterMock {
	return &waiterMock{
		tag:         tag,
		lcPhaseMock: newLcPhaseMock(),
	}
}

func (w *waiterMock) wait() error {
	fmt.Println("Waiting for tag:", w.tag)
	return <-w.exec(nil)
}

type closerMock struct {
	*lcPhaseMock
	tag int
}

func newCloserMock(tag int) *closerMock {
	return &closerMock{
		tag:         tag,
		lcPhaseMock: newLcPhaseMock(),
	}
}

func (c *closerMock) Close() {
	<-c.exec(nil)
}
