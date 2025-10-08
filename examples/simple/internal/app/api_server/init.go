package api_server

import (
	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/internal"
)

func Init() depo.Runner {
	providers := internal.NewProviders()

	return depo.NewRunner(func() {
		providers.HttpApi()
	})
}
