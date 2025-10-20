package cli_history_exporter

import (
	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/cli_history_exporter/cli_handlers"
	"github.com/cardinalby/examples/simple/internal/app/internal"
)

var App func() struct{}

func init() {
	p := internal.Providers

	App = depo.Provide(func() (void struct{}) {
		handler := cli_handlers.NewHistoryExporter(p.UseCases.History())
		depo.UseLifecycle().AddRunnable(handler)
		return void
	})
}
