package depo

import (
	"errors"
	"strings"
)

// ErrAlreadyRunning is returned from Runner.Run if the Runner is already running
var ErrAlreadyRunning = errAlreadyRunning

var errInProvideContext = errors.New("must NOT be called inside `provider` function")

// ErrLifecycleHookFailed is returned from Runner.Run if a lifecycle hook fails to start or wait
// (corresponds to Runner.Run or Starter.Start error)
type ErrLifecycleHookFailed interface {
	error
	LifecycleHook() LifecycleHookInfo
	Unwrap() error
}

type failedLifecyclePhase string

const (
	failedLifecyclePhaseStart failedLifecyclePhase = "start"
	failedLifecyclePhaseWait  failedLifecyclePhase = "wait"
)

func newErrLifecycleHookFailed(
	node *lcNode,
	phase failedLifecyclePhase,
	err error,
) errLifecycleHookFailed {
	return errLifecycleHookFailed{
		lcMethodName:  node.lcHook.getFailedLcMethodName(phase),
		lcNodeOwnInfo: node.lcNodeOwnInfo,
		err:           err,
	}

}

type errLifecycleHookFailed struct {
	lcMethodName  string
	lcNodeOwnInfo lcNodeOwnInfo
	err           error
}

func (e errLifecycleHookFailed) Error() string {
	sb := strings.Builder{}
	sb.WriteString(e.lcMethodName)
	sb.WriteString(" of ")
	sb.WriteString(e.lcNodeOwnInfo.String())
	sb.WriteString("\nfailed: ")
	sb.WriteString(e.err.Error())
	return sb.String()
}

func (e errLifecycleHookFailed) LifecycleHook() LifecycleHookInfo {
	return e.lcNodeOwnInfo
}

func (e errLifecycleHookFailed) Unwrap() error {
	return e.err
}

// ErrUnexpectedRunNilResult is returned from Runner.Run if the Run result is nil
// and OptNilRunResultAsError option is set
var ErrUnexpectedRunNilResult = errors.New("unexpected Run nil result")
