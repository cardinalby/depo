package depo

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cardinalby/depo/internal/dep"
	"github.com/cardinalby/depo/internal/strs"
	"github.com/stretchr/testify/require"
)

// regexp strings
type errMsgRequirement struct {
	lines                 []string
	shouldSkipNextNewline bool
}

func (r *errMsgRequirement) Raw(str string) *errMsgRequirement {
	return r.append(`^` + regexp.QuoteMeta(str) + `$`)
}

func (r *errMsgRequirement) Padding(spaces int) *errMsgRequirement {
	if spaces < 0 {
		panic("spaces should be non-negative")
	}
	padding := strings.Repeat(" ", spaces)
	return r.append(`^` + regexp.QuoteMeta(padding)).skipNextNewLine()
}

func (r *errMsgRequirement) CyclicDependency() *errMsgRequirement {
	return r.Raw(ErrCyclicDependency.Error() + ". " + cyclicDependencyRecommendation)
}

func (r *errMsgRequirement) Panic() *errMsgRequirement {
	return r.append("^panic: ").skipNextNewLine()
}

func (r *errMsgRequirement) Requirements(another *errMsgRequirement) *errMsgRequirement {
	if another == nil || len(another.lines) == 0 {
		return r
	}
	res := &errMsgRequirement{
		lines:                 slices.Clone(r.lines),
		shouldSkipNextNewline: another.shouldSkipNextNewline,
	}
	if r.shouldSkipNextNewline {
		res.lines[len(res.lines)-1] += r.skipLineStartReSymbol(another.lines[0])
		res.lines = append(res.lines, another.lines[1:]...)
	} else {
		res.lines = append(res.lines, another.lines...)
	}
	return res
}

func (r *errMsgRequirement) FileLines(count int) *errMsgRequirement {
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, `^`+regexp.QuoteMeta(strs.MessagesPadding)+`.*?:\d+\s*$`)
	}
	return r.append(lines...)
}

func (r *errMsgRequirement) In() *errMsgRequirement {
	return r.append(`^` + regexp.QuoteMeta(lnFrameNamePrefixIn)).skipNextNewLine()
}

func (r *errMsgRequirement) Arrow() *errMsgRequirement {
	return r.append(`^` + regexp.QuoteMeta(lnFrameNamePrefixArrow)).skipNextNewLine()
}

func (r *errMsgRequirement) Def(depId dep.Id, retType ...string) *errMsgRequirement {
	return r.def("Provide", depId, retType...)
}

func (r *errMsgRequirement) DefErr(depId dep.Id, retType ...string) *errMsgRequirement {
	return r.def("ProvideE", depId, retType...)
}

func (r *errMsgRequirement) def(defCtor string, depId dep.Id, retType ...string) *errMsgRequirement {
	depIdStr := strconv.Itoa(int(depId))
	retTypeRe := `.*?`
	if len(retType) > 1 {
		panic("retType should be a single value, not a slice")
	}
	if len(retType) == 1 {
		retTypeRe = regexp.QuoteMeta(retType[0])
	}
	return r.append(regexp.QuoteMeta(defCtor) + `\(` + depIdStr + `\) ` + retTypeRe + ` @ .+?:\d+$`)
}

func (r *errMsgRequirement) LateInitRegAt(lateInitSeqNum ...int) *errMsgRequirement {
	if len(lateInitSeqNum) > 1 {
		panic("lateInitSeqNum should be a single value, not a slice")
	}
	seqNumRe := ""
	if len(lateInitSeqNum) > 0 && lateInitSeqNum[0] > 1 {
		seqNumRe = regexp.QuoteMeta(fmt.Sprintf("[%d]", lateInitSeqNum))
	}
	return r.append(`LateInit` + seqNumRe + ` registered at:$`)
}

func (r *errMsgRequirement) Of() *errMsgRequirement {
	return r.append(`^Of `).skipNextNewLine()
}

func (r *errMsgRequirement) append(strs ...string) *errMsgRequirement {
	res := &errMsgRequirement{
		lines:                 slices.Clone(r.lines),
		shouldSkipNextNewline: r.shouldSkipNextNewline,
	}
	for _, str := range strs {
		if res.shouldSkipNextNewline {
			res.shouldSkipNextNewline = false
			res.lines[len(r.lines)-1] += r.skipLineStartReSymbol(str)
		} else {
			res.lines = append(res.lines, str)
		}
	}
	return res
}

func (r *errMsgRequirement) skipLineStartReSymbol(re string) string {
	if strings.HasPrefix(re, "^") {
		// if the next line should be skipped, we need to remove the leading ^ from the regexp
		return re[1:] // remove the leading ^
	}
	return re
}

func (r *errMsgRequirement) skipNextNewLine() *errMsgRequirement {
	return &errMsgRequirement{
		lines:                 slices.Clone(r.lines),
		shouldSkipNextNewline: true,
	}
}

func (r *errMsgRequirement) GetReLines() []string {
	return r.lines
}

func checkErrorMsgLines(
	t *testing.T,
	err error,
	expectedLineRegexps *errMsgRequirement,
) {
	t.Helper()
	numerateLines := func(lines []string) string {
		resLines := make([]string, len(lines))
		for i, line := range lines {
			resLines[i] = fmt.Sprintf("%d: %s", i, line)
		}
		return strings.Join(resLines, "\n")
	}

	errLines := strings.Split(err.Error(), "\n")
	for i, expectedLineRegexp := range expectedLineRegexps.lines {
		if i >= len(errLines) {
			t.Fatalf(
				"%d lines are expected but got only %d. \nErr msg:\n%s\n\nRequirements:\n%s",
				len(expectedLineRegexps.lines),
				i,
				numerateLines(errLines),
				numerateLines(expectedLineRegexps.lines),
			)
		}

		require.Regexp(t, expectedLineRegexp, errLines[i],
			"Error line %d does not match. \nErr msg:\n%s\n\nRequirements:\n%s",
			i,
			numerateLines(errLines),
			numerateLines(expectedLineRegexps.lines),
		)
	}
	if len(errLines) > len(expectedLineRegexps.lines) {
		t.Fatalf("Lines starting from %d are unexpected in msg:\n%s",
			len(expectedLineRegexps.lines), numerateLines(errLines),
		)
	}
}
