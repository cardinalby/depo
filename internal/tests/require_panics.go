//go:build depo.testing

package tests

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func RequirePanicsWithErrorIs(
	t *testing.T,
	f func(),
	err error,
	checkErrFns ...func(error),
) {
	RequirePanics(t, f, func(v any) {
		rAsErr, ok := v.(error)
		if !ok {
			t.Fatalf("expected panic with error, but got non-error value: %v", v)
		}
		if !errors.Is(rAsErr, err) {
			t.Fatalf("expected panic with '%v' error, but got '%v'", err, rAsErr)
		}
		for _, checkEmptyE := range checkErrFns {
			checkEmptyE(rAsErr)
		}
	})
}

func RequirePanicsWithErrorAs[E error](
	t *testing.T,
	f func(),
	checkErrFns ...func(E),
) {
	RequirePanics(t, f, func(r any) {
		rAsErr, ok := r.(error)
		if !ok {
			require.Failf(t, "not an error in panic", "%v", r)
		}
		var probe E
		if !errors.As(rAsErr, &probe) {
			require.Failf(
				t,
				"wrong error in panic",
				"expected panic with '%T' error, but got '%v'",
				probe,
				rAsErr,
			)
		} else {
			for _, checkEmptyE := range checkErrFns {
				checkEmptyE(probe)
			}
		}
	})
}

func RequirePanics(
	t *testing.T,
	f func(),
	checkValueFns ...func(any),
) {
	func() {
		defer func() {
			if r := recover(); r == nil {
				require.Fail(t, "expected panic, but got none")
			} else {
				for _, checkValueFn := range checkValueFns {
					checkValueFn(r)
				}
			}
		}()
		f()
	}()
}
