package internal

import (
	"github.com/caarlos0/env/v7"
	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
	"github.com/cardinalby/examples/simple/internal/app/internal/infra/sqlite_migrations"
	"github.com/cardinalby/examples/simple/internal/app/internal/repositories"
	"github.com/cardinalby/examples/simple/internal/app/internal/usecases"
	"github.com/cardinalby/examples/simple/internal/pkg/jsonlog"
	"github.com/cardinalby/examples/simple/internal/pkg/sql"
	"github.com/cardinalby/examples/simple/internal/pkg/sqlite"
)

type InfraProviders struct {
	Config  func() AppConfig
	DB      func() sql.DB
	LogFile func() jsonlog.Logger
}

type ReposProviders struct {
	Cats    func() domain.CatsRepository
	History func() domain.HistoryRepository
}

type UsecasesProviders struct {
	History func() domain.HistoryUsecase
	Cats    func() domain.CatsUsecase
}

// Providers contains shared app dependencies that can be re-used in lazy-manner in different applications
// (see cmd folder). Entry-point components (HTTP server, CLI commands) that are unique per application
// are not defined here (see app/api_server and app/cli_history_exporter folders).
var Providers struct {
	Infra    InfraProviders
	Repos    ReposProviders
	UseCases UsecasesProviders
}

func init() {
	Providers.Infra = newInfraProviders()
	Providers.Repos = newRepoProviders()
	Providers.UseCases = newUsecaseProviders()
}

func newRepoProviders() (repos ReposProviders) {
	// You could've had `infra InfraProviders` argument (instead of global Providers var) to emphasize that
	// repos can only depend on infra providers but not on use-cases

	repos.Cats = depo.Provide(func() domain.CatsRepository {
		return repositories.NewCatsRepository(Providers.Infra.DB())
	})

	repos.History = depo.Provide(func() domain.HistoryRepository {
		return repositories.NewHistoryRepository(Providers.Infra.LogFile())
	})

	return repos
}

func newUsecaseProviders() (useCases UsecasesProviders) {
	useCases.History = depo.Provide(func() domain.HistoryUsecase {
		return usecases.NewHistoryUsecase(
			Providers.Repos.History(),
		)
	})

	useCases.Cats = depo.Provide(func() domain.CatsUsecase {
		return usecases.NewCatsUsecase(
			Providers.Repos.Cats(),
			useCases.History(),
			Providers.Infra.Config().GetOpTimeout(),
		)
	})

	return useCases
}

func newInfraProviders() (infra InfraProviders) {
	infra.Config = depo.Provide(func() AppConfig {
		// It's an exception from lifecycle management. Config is parsed upon creation
		// (not in Start phase of the lifecycle) to be available for other components' constructors
		// and because it's a quick operation that unlikely fails
		var appCfg appConfig
		if err := env.Parse(&appCfg); err != nil {
			panic("failed to parse env vars: " + err.Error())
		}
		return &appCfg
	})

	infra.DB = depo.Provide(func() sql.DB {
		// It's a bit tricky. We want to expose a sql.DB that is ready to be used (has all migrations executed)
		// once Started.

		// Internal `sqliteDB` provider has an own scope (different from `infra.DB`) where
		// it defines lifecycle hooks to open/close DB connection
		sqliteDB := depo.Provide(func() sqlite.Db {
			db := sqlite.NewDb(infra.Config().GetDbFilepath())
			depo.UseLifecycle().AddStarter(db).AddCloser(db)
			return db
		})

		// `infra.DB` depends on `sqliteDB` and defines own lifecycle `Starter` hook based on `migrator.Start`.
		// This way `db := sqlite.NewDb(...)` will start first and only then dependent `migrator.Start` will be called
		db := sqliteDB()
		migrator := sqlite.NewMigrator(db, sqlite_migrations.GetMigrationCollection())
		depo.UseLifecycle().AddStarter(migrator)

		return db
	})

	infra.LogFile = depo.Provide(func() jsonlog.Logger {
		logger := jsonlog.NewLogger(infra.Config().GetLogFilepath())
		depo.UseLifecycle().AddCloser(logger)
		return logger
	})

	return infra
}
