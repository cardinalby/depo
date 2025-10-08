package internal

import (
	"net/http"

	"github.com/caarlos0/env/v7"
	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
	"github.com/cardinalby/examples/simple/internal/app/internal/http_handlers"
	"github.com/cardinalby/examples/simple/internal/app/internal/repositories"
	"github.com/cardinalby/examples/simple/internal/app/internal/usecases"
	"github.com/cardinalby/examples/simple/internal/pkg/httpsrv"
	"github.com/cardinalby/examples/simple/internal/pkg/jsonlog"
	"github.com/cardinalby/examples/simple/internal/pkg/sqlite"
)

type InfraProviders struct {
	Config  func() AppConfig
	Db      func() sqlite.Db
	LogFile func() jsonlog.Logger
}

type ReposProviders struct {
	Cats    func() domain.CatsRepository
	History func() domain.HistoryRepository
}

type UsecasesProviders struct {
	Cats func() domain.CatsUsecase
}

type Providers struct {
	Infra    InfraProviders
	Repos    ReposProviders
	Usecases UsecasesProviders
	HttpApi  func() struct{}
}

func NewProviders() (p Providers) {
	p.Infra = newInfraProviders()
	p.Repos = newRepoProviders(p.Infra)
	p.Usecases = newUsecaseProviders(p.Repos)

	p.HttpApi = depo.Provide(func() (void struct{}) {
		mux := http.NewServeMux()
		http_handlers.RegisterCatsHandlers(mux, p.Usecases.Cats())
		srv := httpsrv.NewServer(p.Infra.Config().GetHttpSrvAddr(), mux)
		depo.UseLifecycle().AddReadinessRunnable(srv)
		return void
	})

	return p
}

func newRepoProviders(p InfraProviders) (rr ReposProviders) {
	rr.Cats = depo.Provide(func() domain.CatsRepository {
		return repositories.NewCatsRepository(p.Db().GetDB())
	})

	rr.History = depo.Provide(func() domain.HistoryRepository {
		return repositories.NewHistoryRepository(p.LogFile())
	})

	return rr
}

func newUsecaseProviders(r ReposProviders) (up UsecasesProviders) {
	up.Cats = depo.Provide(func() domain.CatsUsecase {
		return usecases.NewCatsUsecase(r.Cats(), r.History())
	})

	return up
}

func newInfraProviders() (ip InfraProviders) {
	ip.Config = depo.Provide(func() AppConfig {
		var appCfg appConfig
		if err := env.Parse(appCfg); err != nil {
			panic("failed to parse env vars: " + err.Error())
		}
		return &appCfg
	})

	ip.Db = depo.Provide(func() sqlite.Db {
		return sqlite.NewDb("file:" + ip.Config().GetDbFilepath())
	})

	ip.LogFile = depo.Provide(func() jsonlog.Logger {
		return jsonlog.NewLogger(ip.Config().GetLogFilepath())
	})

	return ip
}
