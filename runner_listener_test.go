package depo

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cardinalby/depo/internal/runtm"
	"github.com/stretchr/testify/require"
)

type runnerListenerMockEventType string

const (
	runnerListenerMockEventTypeStart    runnerListenerMockEventType = "start"
	runnerListenerMockEventTypeReady    runnerListenerMockEventType = "ready"
	runnerListenerMockEventTypeStopping runnerListenerMockEventType = "stopping"
	runnerListenerMockEventTypeDone     runnerListenerMockEventType = "done"
	runnerListenerMockEventTypeShutdown runnerListenerMockEventType = "shutdown"
)

type runnerListenerMockEvent struct {
	moment    time.Time
	eventType runnerListenerMockEventType
	info      LifecycleHook
	err       error
}

type runnerListenerMock struct {
	shouldBeCalledInGoroutineId string
	mutex                       sync.Mutex
	events                      []runnerListenerMockEvent
}

func (l *runnerListenerMock) ShouldBeCalledInCurrentGoroutineId() {
	l.shouldBeCalledInGoroutineId = runtm.GetGoroutineID()
}

func (l *runnerListenerMock) assertCalledInSameGoroutine() {
	if l.shouldBeCalledInGoroutineId != "" && l.shouldBeCalledInGoroutineId != runtm.GetGoroutineID() {
		panic(
			"listener called in different goroutine: expected " + l.shouldBeCalledInGoroutineId +
				", got " + runtm.GetGoroutineID(),
		)
	}
}

func (l *runnerListenerMock) OnStart(info LifecycleHook) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.assertCalledInSameGoroutine()
	l.events = append(l.events, runnerListenerMockEvent{
		moment:    time.Now(),
		eventType: runnerListenerMockEventTypeStart,
		info:      info,
		err:       nil,
	})
}

func (l *runnerListenerMock) OnReady(info LifecycleHook) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.assertCalledInSameGoroutine()
	l.events = append(l.events, runnerListenerMockEvent{
		moment:    time.Now(),
		eventType: runnerListenerMockEventTypeReady,
		info:      info,
		err:       nil,
	})
}

func (l *runnerListenerMock) OnClose(info LifecycleHook, cause error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.assertCalledInSameGoroutine()
	l.events = append(l.events, runnerListenerMockEvent{
		moment:    time.Now(),
		eventType: runnerListenerMockEventTypeStopping,
		info:      info,
		err:       cause,
	})
}

func (l *runnerListenerMock) OnDone(info LifecycleHook, result error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.assertCalledInSameGoroutine()
	l.events = append(l.events, runnerListenerMockEvent{
		moment:    time.Now(),
		eventType: runnerListenerMockEventTypeDone,
		info:      info,
		err:       result,
	})
}

func (l *runnerListenerMock) OnShutdown(cause error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.assertCalledInSameGoroutine()
	l.events = append(l.events, runnerListenerMockEvent{
		moment:    time.Now(),
		eventType: runnerListenerMockEventTypeShutdown,
		info:      nil,
		err:       cause,
	})
}

func (l *runnerListenerMock) requireEventTypes(
	t *testing.T,
	expectedEventTypes ...runnerListenerMockEventType,
) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for i, event := range l.events {
		if !slices.Contains(expectedEventTypes, event.eventType) {
			t.Errorf("event %d: unexpected type %s", i, event.eventType)
		}
	}
}

// tag -> order
func (l *runnerListenerMock) getEventsOrderMap(
	eventType runnerListenerMockEventType,
) map[any]int {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	orderMap := make(map[any]int, len(l.events))
	for _, event := range l.events {
		if event.eventType == eventType {
			tag := event.info.Tag()
			if _, exists := orderMap[tag]; exists {
				panic("duplicate tag in events order map: " + tag.(string))
			}
			orderMap[tag] = len(orderMap)
		}
	}
	return orderMap
}

// tag -> error
func (l *runnerListenerMock) getMockErrorsMap(
	eventType runnerListenerMockEventType,
) map[any]error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	errorsMap := make(map[any]error, len(l.events))
	for _, event := range l.events {
		if event.eventType == eventType {
			if event.info == nil {
				errorsMap[nil] = event.err
			} else {
				errorsMap[event.info.Tag()] = event.err
			}
		}
	}
	return errorsMap
}

func (l *runnerListenerMock) getEvents(eventType runnerListenerMockEventType) []runnerListenerMockEvent {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	var res []runnerListenerMockEvent
	for _, event := range l.events {
		if event.eventType == eventType {
			res = append(res, event)
		}
	}
	return res
}

func (l *runnerListenerMock) requireTagEvent(
	t *testing.T,
	eventType runnerListenerMockEventType,
	tag any,
	checkEvent ...func(event runnerListenerMockEvent),
) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for _, event := range l.events {
		if event.eventType == eventType && event.info.Tag() == tag {
			for _, check := range checkEvent {
				check(event)
			}
			return
		}
	}
	t.Errorf("no event of type %s for tag %v", eventType, tag)
}

// can use * -> error to point to all components
// use "tag -> error" to point to specific components
// value can be error or func(err error, msgAndArgs []any) that validates the error
type errorRequirements map[any]any
type listenerEventTypeErrRequirements map[runnerListenerMockEventType]errorRequirements

func (er errorRequirements) getErrorValidator(t *testing.T, tag any) func(err error, msgAndArgs []any) {
	makeValidator := func(it any) func(err error, msgAndArgs []any) {
		if itErr, ok := it.(error); ok {
			return func(err error, msgAndArgs []any) {
				require.ErrorIs(t, err, itErr, msgAndArgs...)
			}
		}
		if itFn, ok := it.(func(error, []any)); ok {
			return itFn
		}
		if it == nil {
			return func(err error, msgAndArgs []any) {
				require.NoError(t, err, msgAndArgs...)
			}
		}
		t.Fatalf("unexpected error requirement type %T for tag %v", it, tag)
		return nil
	}
	if r, ok := er[tag]; ok {
		return makeValidator(r)
	}
	if r, ok := er[any("*")]; ok {
		return makeValidator(r)
	}
	return nil
}

func (l *runnerListenerMock) requireErrors(
	t *testing.T,
	eventTypeErrRequirements listenerEventTypeErrRequirements,
) {
	for eventType, req := range eventTypeErrRequirements {
		errorsMap := l.getMockErrorsMap(eventType)
		for tag, err := range errorsMap {
			req.getErrorValidator(t, tag)(err, []any{"eventType: %s, tag: %v", eventType, tag})
		}
	}
}
