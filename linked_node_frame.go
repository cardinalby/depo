package depo

import (
	"strconv"

	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/strs"
)

type lnFramesChainStrOpt uint8

const (
	lnFramesChainStrOptNormal lnFramesChainStrOpt = iota
	lnFramesChainStrOptSkipFirstDepInfo
	lnFramesChainStrOptCycle
)

const lnFrameNamePrefixIn = "In "
const lnFrameNamePrefixArrow = "—→ "
const lnFrameNamePrefixLen = 3 // != len(lnFrameNamePrefixIn) because of UTF-8 characters

// linkedNodeFrame represents a frame in the nodeFramesStack that has a pointer to a next frame.
// The chain of linkedNodeFrames constitutes a linked list of frames that is used in lateInit errors:
// each node has a pointer to own part of the list that leads to the lateInit frame causing the error.
// Using this structure allows to avoid copying the whole stack to store individual errors for all dependents
// of the failed node
type linkedNodeFrame struct {
	depNode depNode
	// 0 for non-lateInit frames
	lateInitSeqNum int
	// nil for non-lateInit frames
	lateInitRegAt runtm.CallerCtxs

	// next frame that is higher in the stack (was called by the current frame).
	next *linkedNodeFrame
	// user-code lines that called `next` inside the current frame or lead to panic in the current frame
	nextCallerCtxs runtm.CallerCtxs
}

func (lf *linkedNodeFrame) ChainString(namesRendering lnFramesChainStrOpt) string {
	// we go from lower to higher frames, so we need to collect them in reverse order
	var reversedLines strs.Lines
	isFirst := true
	var stopAt *linkedNodeFrame
	for current := lf; current != nil; current = current.next {
		stopAtCurrent := false
		if isFirst {
			isFirst = false
			if namesRendering == lnFramesChainStrOptCycle {
				stopAt = current
				reversedLines.Push(lnFrameNamePrefixIn + current.String(true))
			} else if namesRendering == lnFramesChainStrOptSkipFirstDepInfo {
				if lateInitRegString := current.String(false); lateInitRegString != "" {
					reversedLines.Push(lnFrameNamePrefixIn + lateInitRegString)
				}
			} else {
				reversedLines.Push(lnFrameNamePrefixIn + current.String(true))
			}
		} else {
			stopAtCurrent = stopAt == current
			namePrefix := lnFrameNamePrefixIn
			if stopAtCurrent {
				namePrefix = lnFrameNamePrefixArrow
			}
			reversedLines.Push(namePrefix + current.String(true))
		}
		if stopAtCurrent {
			break
		}
		if len(current.nextCallerCtxs) > 0 {
			reversedLines.Push(current.nextCallerCtxs.FileLines())
		}
	}
	return reversedLines.JoinReverse()
}

func (lf *linkedNodeFrame) String(printDepInfo bool) string {
	var lines strs.Lines
	if lf.lateInitSeqNum > 0 {
		liName := "LateInit"
		if lf.lateInitSeqNum > 1 {
			liName += "[" + strconv.Itoa(lf.lateInitSeqNum) + "]"
		}
		liName += " registered at:"
		lines.Push(liName)
		lines.Push(lf.lateInitRegAt.FileLines())
		if printDepInfo {
			lines.Push("Of " + lf.depNode.GetDepInfo().String())
		}
	} else if printDepInfo {
		lines.Push(lf.depNode.GetDepInfo().String())
	} else {
		return ""
	}
	return lines.Join()
}

func (lf *linkedNodeFrame) FindNodeFrameInChain(node depNode) *linkedNodeFrame {
	for current, isFirstIt := lf, true; current != nil; current = current.next {
		if isFirstIt {
			isFirstIt = false
		} else if current == lf {
			// cycle detected, stop searching
			return nil
		}
		if current.depNode == node {
			return current
		}
	}
	return nil
}
