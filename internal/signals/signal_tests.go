//go:build depo.testing

package signals

import (
	"os"
	"slices"
	"sync"
)

var mockSubscribersMu sync.RWMutex
var mockSubscribers = make(map[chan<- os.Signal][]os.Signal)

func MockSubscribersCount() int {
	mockSubscribersMu.RLock()
	defer mockSubscribersMu.RUnlock()
	return len(mockSubscribers)
}

func SendMockSignal(sig os.Signal) {
	mockSubscribersMu.RLock()
	defer mockSubscribersMu.RUnlock()
	for c, signals := range mockSubscribers {
		if slices.Contains(signals, sig) {
			select {
			case c <- sig:
			default:
			}
		}
	}
}

func Notify(c chan<- os.Signal, sig ...os.Signal) {
	mockSubscribersMu.Lock()
	defer mockSubscribersMu.Unlock()
	mockSubscribers[c] = append(mockSubscribers[c], sig...)
}

func Stop(c chan<- os.Signal) {
	mockSubscribersMu.Lock()
	defer mockSubscribersMu.Unlock()
	delete(mockSubscribers, c)
}
