package depo

import "time"

// StarterOption is an option that can be added to Starter definition in UseLifecycle().AddStarter()
type StarterOption interface {
	applyToStarterCfg(cfg *starterCfg)
}

type RunnableOption interface {
	applyToRunnableCfg(cfg *runnableCfg)
}

type ReadinessRunnableOption interface {
	applyToReadinessRunnableCfg(cfg *readinessRunnableCfg)
}

type RunnerOption interface {
	applyToRunnerCfg(cfg *runnerCfg)
}

type starterCfg struct {
	startTimeout time.Duration
}

type waiterCfg struct {
	nilResultAsError bool
}

type runnableCfg struct {
	waiterCfg
}

type readinessRunnableCfg struct {
	starterCfg
	runnableCfg
}

type runnerCfg struct {
	readinessRunnableCfg
	runnerListeners runnerListeners
}

type optStartTimeout time.Duration

func (o optStartTimeout) applyToStarterCfg(cfg *starterCfg) {
	cfg.startTimeout = time.Duration(o)
}

func (o optStartTimeout) applyToReadinessRunnableCfg(cfg *readinessRunnableCfg) {
	cfg.startTimeout = time.Duration(o)
}

func (o optStartTimeout) applyToRunnerCfg(cfg *runnerCfg) {
	cfg.startTimeout = time.Duration(o)
}

// OptStartTimeout specifies the timeout for a component to become ready. This option can be passed to:
//   - UseLifecycle().AddStarter(): instructs the Runner to use a context with timeout when calling Start()
//   - UseLifecycle().AddReadinessRunnable(): instructs the Runner to cancel the context passed to Run() method
//     with cause context.DeadlineExceeded after the timeout.
//   - NewRunner(): instructs the Runner to apply the timeout by default to all ReadinessRunnable and Starter components
//     (if they don't have their own timeout configured) as described above.
//
// If you need to set the timeout for a Runner itself to become ready instead, just pass a context with timeout to Runner.Run
func OptStartTimeout(timeout time.Duration) interface {
	StarterOption
	ReadinessRunnableOption
	RunnerOption
} {
	return optStartTimeout(timeout)
}

type optRunnerListeners []RunnerListener

// OptRunnerListeners can be used to pass to NewRunner to add listeners to the Runner
func OptRunnerListeners(listeners ...RunnerListener) RunnerOption {
	return optRunnerListeners(listeners)
}

func (o optRunnerListeners) applyToRunnerCfg(cfg *runnerCfg) {
	cfg.runnerListeners = append(cfg.runnerListeners, o...)
}

type optNilRunResultAsError bool

func (o optNilRunResultAsError) applyToRunnableCfg(cfg *runnableCfg) {
	cfg.nilResultAsError = bool(o)
}

func (o optNilRunResultAsError) applyToReadinessRunnableCfg(cfg *readinessRunnableCfg) {
	cfg.nilResultAsError = bool(o)
}

func (o optNilRunResultAsError) applyToRunnerCfg(cfg *runnerCfg) {
	cfg.nilResultAsError = bool(o)
}

// OptNilRunResultAsError can be used to modify the default Runner behavior of handling nil results received from
// Run() method of Runnable / ReadinessRunnable component. If set, nil Run result will be considered an error and
// will trigger shutdown with ErrUnexpectedRunNilResult.
// If passed to AddRunnable() / AddReadinessRunnable() method of UseLifecycle() it will affect only that single
// Runnable / ReadinessRunnable.
// If passed to NewRunner() it will affect all Runnable / ReadinessRunnable components
func OptNilRunResultAsError() interface {
	RunnableOption
	ReadinessRunnableOption
	RunnerOption
} {
	return optNilRunResultAsError(true)
}

func newStarterCfg(opts []StarterOption) (cfg starterCfg) {
	for _, opt := range opts {
		opt.applyToStarterCfg(&cfg)
	}
	return cfg
}

func newRunnableCfg(opts []RunnableOption) (cfg runnableCfg) {
	for _, opt := range opts {
		opt.applyToRunnableCfg(&cfg)
	}
	return cfg
}

func newReadinessRunnableCfg(opts []ReadinessRunnableOption) (cfg readinessRunnableCfg) {
	for _, opt := range opts {
		opt.applyToReadinessRunnableCfg(&cfg)
	}
	return cfg
}

func newRunnerCfg(opts []RunnerOption) (cfg runnerCfg) {
	for _, opt := range opts {
		opt.applyToRunnerCfg(&cfg)
	}
	return cfg
}
