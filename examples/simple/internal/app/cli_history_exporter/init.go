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
		// Use depo.OptNilRunResultAsError() option to make Runner return an `depo.ErrUnexpectedRunNilResult` error
		// when the handler completes successfully (i.e. returns nil). We do that in order to allow the Runner
		// to finish (because another lifecycle-aware components are blocking it)
		depo.UseLifecycle().AddRunnable(handler, depo.OptNilRunResultAsError())
		return void
	})

	return depo.NewRunner(func() {
		// other tasks can be added here
		cliHandler()
	})
}
