package depo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUseComponentID(t *testing.T) {
	require.Equal(t, uint64(0), UseComponentID())
}
