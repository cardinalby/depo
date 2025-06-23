//go:build !depo.testing

package signals

import (
	"os"
	"os/signal"
)

//goland:noinspection GoUnusedExportedFunction
func MockSubscribersCount() int {
	return 0
}

//goland:noinspection GoUnusedExportedFunction
func SendMockSignal(_ os.Signal) {}

func Notify(c chan<- os.Signal, sig ...os.Signal) {
	signal.Notify(c, sig...)
}

func Stop(c chan<- os.Signal) {
	signal.Stop(c)
}
