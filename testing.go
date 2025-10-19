package depo

import (
	"sync"
	"sync/atomic"

	"github.com/cardinalby/depo/internal/dep"
	"github.com/cardinalby/depo/internal/tests"
)

var testingGlobalVarsMu sync.Mutex

// for testing purposes, keep track of created nodes
var createdNodes map[dep.Id]depNode
var lastRunnableEventSeqNo atomic.Int64

func init() {
	if tests.IsTestingBuild {
		createdNodes = make(map[dep.Id]depNode)
	}
}
