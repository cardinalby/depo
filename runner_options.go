package depo

type runnerCfg struct {
	listeners              runnerListeners
	shutDownOnNilRunResult bool
}

// RunnerOption allows customization of the Runner's behavior
type RunnerOption func(*runnerCfg)

// RunnerOptListeners allows you to add listeners to the Runner
func RunnerOptListeners(listeners ...RunnerListener) RunnerOption {
	return func(cfg *runnerCfg) {
		cfg.listeners = append(cfg.listeners, listeners...)
	}
}

// RunnerOptShutDownOnNilRunResult configures the Runner to shut down if a runner returns nil
// (normally it's ignored until all components are done. In this case the Runner returns nil as well)
func RunnerOptShutDownOnNilRunResult() RunnerOption {
	return func(cfg *runnerCfg) {
		cfg.shutDownOnNilRunResult = true
	}
}

func newRunnerCfg(opts []RunnerOption) runnerCfg {
	cfg := runnerCfg{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
