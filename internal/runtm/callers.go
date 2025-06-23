package runtm

import (
	"math"
	"runtime"
	"strconv"
	"strings"

	"github.com/cardinalby/depo/internal/strs"
	"github.com/cardinalby/depo/internal/tests"
)

// CallerLevel says how far from the current call client code CallerCtxs is
// 0 means in the function itself, 1 means in the CallerCtxs of the function, etc.
type CallerLevel int

// PanicRecoveryDeferFnCallerLevel is a CallerLevel to be passed to FindClosestKnownCallerCtxInCallStack from defer
// function that recovers from a panic to get user code lines that called the panic.
// One frame is defer function itself, the second is go's runtime panic handler that calls the defer function
const PanicRecoveryDeferFnCallerLevel CallerLevel = 2

// CallerCtx represents the context of a function call. It's a lightweight runtime.Frame analog
type CallerCtx struct {
	function string
	file     string
	line     int
}

// NewCallerCtx creates a new CallerCtx based on the current call stack.
// CallerLevel == 0 points to a function that calls NewCallerCtx directly,
// CallerLevel == 1 points to a function that calls the function that calls NewCallerCtx, and so on.
func NewCallerCtx(callerLevel CallerLevel) CallerCtx {
	return newCallerCtxFromFrame(getCallerFrame(callerLevel + 1))
}

func newCallerCtxFromFrame(frame *runtime.Frame) CallerCtx {
	return CallerCtx{
		function: frame.Function,
		file:     frame.File,
		line:     frame.Line,
	}
}

func (c CallerCtx) Empty() bool {
	return c.function == ""
}

func (c CallerCtx) Function() string {
	return c.function
}

func (c CallerCtx) File() string {
	return c.file
}

func (c CallerCtx) Line() int {
	return c.line
}

func (c CallerCtx) String() string {
	return c.stringImpl(" @ ")
}

func (c CallerCtx) StringGoFormat() string {
	return c.stringImpl("\n    ")
}

func (c CallerCtx) stringImpl(fileLineDelim string) string {
	if c.function == "" && c.file == "" && c.line == 0 {
		return "<empty>"
	}
	var sb strings.Builder
	if c.function == "" {
		sb.WriteString("<unknown>")
	} else {
		sb.WriteString(c.function)
	}
	sb.WriteString(fileLineDelim)
	sb.WriteString(c.FileLine())
	return sb.String()
}

func (c CallerCtx) FileLine() string {
	if c.file == "" && c.line == 0 {
		return "<unknown>"
	}
	sb := strings.Builder{}
	sb.WriteString(c.file)
	sb.WriteString(":")
	sb.WriteString(strconv.Itoa(c.line))
	return sb.String()
}

// callerSearchBatchSize is the number of stack frames to search in each iteration.
// Provide dependencies should be mostly one frame deep
const callerSearchBatchSize = 10

// IsNotInCallStackCallerLevel skips all the frames and reports that CallerCtx
// // is not in the call stack if passed to FindClosestKnownCallerCtxInCallStack,
const IsNotInCallStackCallerLevel = CallerLevel(math.MinInt)

// FindClosestKnownCallerCtxInCallStack searches for the closest CallerCtx in the call stack that matches one
// of the knownCallerCtxs.
// `CallerLevel` is used to skip not relevant frames in the call stack.
// `userCallerCtxs` result is the frames of the user code we encountered in the call stack before the first
// `knownCallerCtxs` frame.
func FindClosestKnownCallerCtxInCallStack(
	callerLevel CallerLevel,
	knownCallerCtxs ...CallerCtx,
) (
	userCallerCtxs CallerCtxs,
	foundCallerCtx CallerCtx,
	isFound bool,
) {
	if len(knownCallerCtxs) == 0 || callerLevel < 0 {
		return nil, foundCallerCtx, false
	}
	skip := int(callerLevel) + 2
	for {
		callers := make([]uintptr, callerSearchBatchSize)
		n := runtime.Callers(skip, callers)
		if n == 0 {
			return nil, foundCallerCtx, false
		}
		frames := runtime.CallersFrames(callers)
		for i := 0; i < n; i++ {
			frame, more := frames.Next()
			// we don't expect more than 2 knownCallerCtxs in the call stack, so don't
			// create a "function -> CallerCtx" search map for them
			for _, knownCallerCtx := range knownCallerCtxs {
				if frame.Function == knownCallerCtx.function {
					foundCallerCtx = knownCallerCtx
					return userCallerCtxs, foundCallerCtx, true
				}
			}
			if !isLibInternalsRuntimeFrame(&frame) {
				userCallerCtxs = append(userCallerCtxs, newCallerCtxFromFrame(&frame))
			}
			if !more && i < n-1 {
				return nil, foundCallerCtx, false
			}
		}

		skip += callerSearchBatchSize
	}
}

// CallerLevel == 0 points to a function that calls getCallerFrame directly,
// CallerLevel == 1 points to a function that calls the function that calls getCallerFrame, and so on.
func getCallerFrame(callerLevel CallerLevel) *runtime.Frame {
	callers := make([]uintptr, 1)
	n := runtime.Callers(2+int(callerLevel), callers)
	if n == 0 {
		panic("no callers found")
	}
	frames := runtime.CallersFrames(callers)
	frame, _ := frames.Next()
	return &frame
}

const packageName = "github.com/cardinalby/depo"

func isLibInternalsRuntimeFrame(frame *runtime.Frame) bool {
	//goland:noinspection GoBoolExpressions
	if tests.IsTestingBuild &&
		(strings.HasSuffix(frame.File, "_test.go") || strings.HasPrefix(frame.Function, packageName+"/example")) {
		// Report test files as not being part of the library internals
		return false
	}
	return strings.HasPrefix(frame.Function, packageName)
}

type CallerCtxs []CallerCtx

func (ctxs CallerCtxs) FileLines() string {
	var lines strs.Lines
	for _, callerCtx := range ctxs {
		lines.Push(strs.MessagesPadding + callerCtx.FileLine())
	}
	return lines.Join()
}
