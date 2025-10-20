package api_server

import (
	"net/http"

	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/api_server/http_handlers"
	"github.com/cardinalby/examples/simple/internal/app/internal"
	"github.com/cardinalby/examples/simple/internal/pkg/httpsrv"
)

var App func() struct{}

func init() {
	App = depo.Provide(func() (void struct{}) {
		p := &internal.Providers
		mux := http.NewServeMux()
		http_handlers.RegisterCatsHandlers(mux, p.UseCases.Cats())
		srv := httpsrv.NewServer(p.Infra.Config().GetHttpSrvAddr(), mux)
		depo.UseLifecycle().AddReadinessRunnable(srv)

		return void
	})
}
