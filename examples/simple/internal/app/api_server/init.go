package api_server

import (
	"net/http"

	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/api_server/http_handlers"
	"github.com/cardinalby/examples/simple/internal/app/internal"
	"github.com/cardinalby/examples/simple/internal/pkg/httpsrv"
)

func Init() depo.Runner {
	p := internal.NewProviders()

	httpApiServer := depo.Provide(func() (void struct{}) {
		mux := http.NewServeMux()
		http_handlers.RegisterCatsHandlers(mux, p.UseCases.Cats())
		srv := httpsrv.NewServer(p.Infra.Config().GetHttpSrvAddr(), mux)
		depo.UseLifecycle().AddReadinessRunnable(srv)
		return void
	})

	return depo.NewRunner(func() {
		// other tasks can be added here (e.g. consuming from a message queue, etc.)

		// We are not interested in the return value, just want to make sure we requested it - in turn, it
		// creates all the dependencies it needs (but not extra ones that are not needed for the HTTP API)
		// and registers required lifecycle hooks that will be executed in the right order when Runner.Run is called
		httpApiServer()
	})
}
