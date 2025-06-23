package depo

import (
	"testing"
)

func requireNoRunnableNodes(t *testing.T) {
	t.Helper()
	for _, node := range createdNodes {
		if node.GetRegState() == nodeRegStateWithLcHooks {
			t.Errorf("%s node has direct or transitive runnables", node.GetDepInfo().String())
		}
	}
}
