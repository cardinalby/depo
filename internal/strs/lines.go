package strs

import "strings"

const newLineSeparator = "\n"

type LinesReader interface {
	Count() int
	Lines() []string
	Join() string
	JoinReverse() string
}

type Lines struct {
	lines     []string
	totalSize int
}

//goland:noinspection GoMixedReceiverTypes
func (b Lines) Count() int {
	return len(b.lines)
}

//goland:noinspection GoMixedReceiverTypes
func (b Lines) Lines() []string {
	return b.lines
}

//goland:noinspection GoMixedReceiverTypes
func (b *Lines) Push(lines ...string) *Lines {
	b.lines = append(b.lines, lines...)
	for _, line := range lines {
		b.totalSize += len(line)
	}
	return b
}

//goland:noinspection GoMixedReceiverTypes
func (b *Lines) PushReversed(lines ...string) *Lines {
	for i := len(lines) - 1; i >= 0; i-- {
		b.lines = append(b.lines, lines[i])
		b.totalSize += len(lines[i])
	}
	return b
}

//goland:noinspection GoMixedReceiverTypes
func (b Lines) JoinReverse() string {
	if len(b.lines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(b.totalSize + len(b.lines) - 1) // +len(b.lines)-1 for the newline characters
	for i := len(b.lines) - 1; i >= 0; i-- {
		if i < len(b.lines)-1 {
			sb.WriteString(newLineSeparator)
		}
		sb.WriteString(b.lines[i])
	}
	return sb.String()
}

//goland:noinspection GoMixedReceiverTypes
func (b Lines) Join() string {
	if len(b.lines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(b.totalSize + len(b.lines) - 1) // +len(b.lines)-1 for the newline characters
	for i, line := range b.lines {
		if i > 0 {
			sb.WriteString(newLineSeparator)
		}
		sb.WriteString(line)
	}
	return sb.String()
}
