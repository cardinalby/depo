package depo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testDepInterface interface {
	OwnValue() int
	OwnPlusDepsValue() int
}

type testLateInitDep struct {
	dep      testDepInterface
	ownValue int
}

func (d *testLateInitDep) OwnValue() int {
	return d.ownValue
}

func (d *testLateInitDep) OwnPlusDepsValue() int {
	return d.ownValue*10 + d.dep.OwnValue()
}

func TestUsePtrLateInit(t *testing.T) {
	t.Cleanup(testsCleanup)

	var def1, def2 func() testDepInterface

	def1 = Provide(func() testDepInterface {
		return UsePtrLateInit(func() *testLateInitDep {
			return &testLateInitDep{
				ownValue: 1,
				dep:      def2(),
			}
		})
	})

	def2 = Provide(func() testDepInterface {
		return &testLateInitDep{
			ownValue: 2,
			dep:      def1(),
		}
	})

	require.Equal(t, def1().OwnValue(), 1)
	require.Equal(t, def1().OwnPlusDepsValue(), 12)
	require.Equal(t, def2().OwnValue(), 2)
	require.Equal(t, def2().OwnPlusDepsValue(), 21)
}
