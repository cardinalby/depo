package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/cardinalby/depo"

	"github.com/cardinalby/depo/example/internal/components"
	"github.com/cardinalby/depo/example/internal/domain"
)

type graph struct {
	ctx                      context.Context
	cancel                   context.CancelFunc
	reg                      *components.Registry
	runner                   depo.Runner
	isShutDownOnNilRunResult bool
	runnerStatus             domain.ComponentStatus
	isResetting              bool
	dependOnMap              map[uint64][]uint64
	runnerErr                error
	done                     chan struct{}
}

func newGraph(listener depo.RunnerListener, shutDownOnNilRunResult bool) *graph {
	ctx, cancel := context.WithCancel(context.Background())
	reg := components.NewRegistry()
	c := components.GetComponents(reg)
	options := []depo.RunnerOption{
		depo.OptRunnerListeners(listener),
	}
	if shutDownOnNilRunResult {
		options = append(options, depo.OptNilRunResultAsError())
	}
	runner, err := depo.NewRunnerE(func() error {
		c.A()
		c.B()
		c.C()
		c.J()
		return nil
	}, options...)
	if err != nil {
		panic(fmt.Errorf("failed to create runner: %w", err))
	}
	dependOnMap := make(map[uint64][]uint64)
	var visit func(h depo.LifecycleHookNode)
	visit = func(h depo.LifecycleHookNode) {
		var dependsOnIds []uint64
		for _, dep := range h.DependsOnHooks() {
			dependsOnIds = append(dependsOnIds, dep.ComponentInfo().ID())
			visit(dep)
		}
		dependOnMap[h.ComponentInfo().ID()] = dependsOnIds
	}
	for _, h := range runner.GetRootLifecycleHookNodes() {
		visit(h)
	}

	return &graph{
		ctx:                      ctx,
		cancel:                   cancel,
		reg:                      reg,
		runner:                   runner,
		isShutDownOnNilRunResult: shutDownOnNilRunResult,
		runnerStatus:             domain.StatusPending,
		dependOnMap:              dependOnMap,
		done:                     make(chan struct{}),
	}
}

type Usecase struct {
	logFn func(format string, args ...any)
	g     *graph
	mu    sync.Mutex
}

func NewUsecase(
	logFn func(format string, args ...any),
) *Usecase {
	uc := &Usecase{
		logFn: logFn,
	}
	uc.g = newGraph(uc, false)
	return uc
}

func (u *Usecase) Graph() (res domain.Graph) {
	u.mu.Lock()
	defer u.mu.Unlock()
	res.Status = u.g.runnerStatus
	res.ShutDownOnNilRunResult = u.g.isShutDownOnNilRunResult

	if u.g != nil {
		res.RunnerError = u.formatErr(u.g.runnerErr)
	}

	for id, c := range u.g.reg.GetAll() {
		dc := domain.Component{
			ID:        id,
			Name:      c.GetName(),
			DependsOn: u.g.dependOnMap[id],
			Status:    domain.StatusNonRunnable,
		}
		if rc, ok := c.(components.RunnableComponent); ok {
			state := rc.GetState()
			dc.Status = state.Status
			if state.DoneErr != nil {
				dc.DoneError = u.formatErr(state.DoneErr)
			}
			cfg := rc.GetConfig()
			dc.StartError = cfg.StartErr
			dc.Delay = cfg.Delay
		}
		res.Components = append(res.Components, dc)
	}
	slices.SortFunc(res.Components, func(a, b domain.Component) int {
		return int(a.ID) - int(b.ID)
	})

	return res
}

func (u *Usecase) StartAll() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.g.runnerStatus != domain.StatusPending {
		return fmt.Errorf("runner is %s", u.g.runnerStatus)
	}
	u.g.runnerStatus = domain.StatusStarting

	go func() {
		onReady := func() {
			u.mu.Lock()
			defer u.mu.Unlock()
			u.g.runnerStatus = domain.StatusReady
			u.logFn("Runner is ready")
		}
		err := u.g.runner.Run(u.g.ctx, onReady)
		u.mu.Lock()
		defer u.mu.Unlock()
		u.g.runnerStatus = domain.StatusDone
		u.g.runnerErr = err
		close(u.g.done)
	}()

	return nil
}

func (u *Usecase) StopAll() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.g.runnerStatus == domain.StatusPending {
		u.logFn("Runner is not started, nothing to stop")
		return nil
	}
	u.g.cancel()
	return nil
}

func (u *Usecase) Reset(shutDownOnNilRunResult bool) {
	u.mu.Lock()
	if u.g == nil || u.g.isResetting {
		return
	}
	u.g.isResetting = true

	switch u.g.runnerStatus {
	case domain.StatusStarting, domain.StatusReady:
		for _, c := range u.g.reg.GetAll() {
			if rc, ok := c.(components.RunnableComponent); ok {
				rc.Complete(context.Canceled)
			}
		}
		u.g.cancel()
	case domain.StatusPending:
		close(u.g.done)
	}
	done := u.g.done
	u.mu.Unlock()
	<-done

	u.mu.Lock()
	u.g = newGraph(u, shutDownOnNilRunResult)
	u.mu.Unlock()
}

func (u *Usecase) UpdateComponent(compID uint64, startErr string, delay time.Duration) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	c := u.g.reg.Get(compID)
	if c == nil {
		return fmt.Errorf("component with ID %d not found", compID)
	}
	rc := c.(components.RunnableComponent)
	if rc == nil {
		return fmt.Errorf("is not RunnableComponent %d", compID)
	}
	rc.SetConfig(components.RunnableComponentConfig{
		StartErr: startErr,
		Delay:    delay,
	})
	return nil
}

func (u *Usecase) StopComponent(compID uint64, withErr bool) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	c := u.g.reg.Get(compID)
	rc := c.(components.RunnableComponent)
	if rc == nil {
		return fmt.Errorf("RunnableComponent with ID %d not found", compID)
	}
	if state := c.GetState(); state.Status == domain.StatusPending || state.Status == domain.StatusDone {
		return nil
	}
	if withErr {
		rc.Complete(errors.New("some_err"))
	} else {
		rc.Complete(nil)
	}
	return nil
}

func (u *Usecase) OnStart(lcHook depo.LifecycleHookInfo) {
	u.logFn("listener.OnStart(%v)\n", lcHook.ComponentInfo().ID())
}

func (u *Usecase) OnReady(lcHook depo.LifecycleHookInfo) {
	u.logFn("listener.OnReady(%v)\n", lcHook.ComponentInfo().ID())
}

func (u *Usecase) OnClose(lcHook depo.LifecycleHookInfo, cause error) {
	u.logFn("listener.OnClose(%v, %v)\n", lcHook.ComponentInfo().ID(), u.formatErr(cause))
}

func (u *Usecase) OnDone(lcHook depo.LifecycleHookInfo, result error) {
	u.logFn("listener.OnDone(%v, %v)\n", lcHook.ComponentInfo().ID(), u.formatErr(result))
}

// OnShutdown is called once when the Runner is shutting down, either due to an error in one of the components
// (see ErrLifecycleHookFailed) or a shutdown due to context cancellation (returns context.Canceled error)
func (u *Usecase) OnShutdown(cause error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.g.runnerStatus = domain.StatusClosing
	u.logFn("listener.OnShutdown(%v)\n", u.formatErr(cause))
}

func (u *Usecase) formatErr(err error) string {
	if err == nil {
		return "nil"
	}
	if errors.Is(err, context.Canceled) {
		return "context.Canceled"
	}
	var componentErr depo.ErrLifecycleHookFailed
	if errors.As(err, &componentErr) {
		return fmt.Sprintf("%v failed: %s",
			componentErr.LifecycleHook().Tag(), u.formatErr(componentErr.Unwrap()))
	}
	return err.Error()
}
