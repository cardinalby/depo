package cli_history_exporter

import (
	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/cli_history_exporter/cli_handlers"
	"github.com/cardinalby/examples/simple/internal/app/internal"
)

func Init() depo.Runner {
	p := internal.NewProviders()

	cliHandler := depo.Provide(func() (void struct{}) {
		handler := cli_handlers.NewHistoryExporter(p.UseCases.History())
		depo.UseLifecycle().AddRunnable(handler)
		return void
	})

	return depo.NewRunner(func() {
		// other tasks can be added here
		cliHandler()
	})
}
