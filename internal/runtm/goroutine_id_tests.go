//go:build depo.testing

package runtm

import (
	"runtime"
	"strings"
)

func GetGoroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	line := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	fields := strings.Fields(line)
	return fields[0]
}
