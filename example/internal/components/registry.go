package components

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cardinalby/depo"
	"github.com/cardinalby/depo/example/internal/domain"
)

type RunnableComponentConfig struct {
	StartErr string
	Delay    time.Duration
}

type ComponentState struct {
	Status  domain.ComponentStatus
	DoneErr error
}

type Component interface {
	GetName() string
	GetState() ComponentState
}

type RunnableComponent interface {
	Component
	SetConfig(cfg RunnableComponentConfig)
	GetConfig() RunnableComponentConfig
	Complete(err error)
	depo.ReadinessRunnable
}

type runnableComponent struct {
	name    string
	mu      sync.RWMutex
	state   ComponentState
	cfg     RunnableComponentConfig
	waitErr chan error
}

func (cn *runnableComponent) GetName() string {
	return cn.name
}

func (cn *runnableComponent) GetState() ComponentState {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	return cn.state
}

func (cn *runnableComponent) GetConfig() RunnableComponentConfig {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	return cn.cfg
}

func (cn *runnableComponent) SetConfig(cfg RunnableComponentConfig) {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	cn.cfg = cfg
}

func (cn *runnableComponent) setStatus(status domain.ComponentStatus) {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	cn.state.Status = status
}

func (cn *runnableComponent) setDoneErr(err error) {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	cn.state.DoneErr = err
}

func (cn *runnableComponent) Run(ctx context.Context, onReady func()) error {
	defer func() {
		fmt.Printf("RunnableComponent(%s).Run returned\n", cn.name)
	}()
	cfg := cn.GetConfig()
	cn.setStatus(domain.StatusStarting)

	if cfg.StartErr != "" {
		<-time.After(cfg.Delay / 2)
		cn.setStatus(domain.StatusDone)
		err := errors.New(cfg.StartErr)
		cn.setDoneErr(err)
		return err
	}

	// start
	select {
	case <-time.After(cfg.Delay):
		cn.setStatus(domain.StatusReady)
		onReady()

	case err := <-cn.waitErr:
		cn.setStatus(domain.StatusDone)
		cn.setDoneErr(err)
		return err

	case <-ctx.Done():
		cn.setStatus(domain.StatusDone)
		cn.setDoneErr(ctx.Err())
		return ctx.Err()
	}

	// wait
	cfg = cn.GetConfig()
	select {
	case err := <-cn.waitErr:
		cn.setStatus(domain.StatusDone)
		cn.setDoneErr(err)
		return err
	case <-ctx.Done():
		cn.setStatus(domain.StatusClosing)
		time.Sleep(cfg.Delay)
		cn.setStatus(domain.StatusDone)
		cn.setDoneErr(ctx.Err())
		return ctx.Err()
	}
}

func (cn *runnableComponent) Complete(err error) {
	cn.waitErr <- err
}

type component struct {
	name string
}

func (c *component) GetName() string {
	return c.name
}

func (c *component) GetState() ComponentState {
	return ComponentState{
		Status: domain.StatusNonRunnable,
	}
}

type Registry struct {
	mu sync.RWMutex
	// component ID -> state
	components map[uint64]Component
}

func NewRegistry() *Registry {
	return &Registry{
		components: make(map[uint64]Component),
	}
}

func (o *Registry) NewRunnable(componentID uint64, name string) RunnableComponent {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, exists := o.components[componentID]; exists {
		panic("only one runnable per component is allowed in the example app")
	}
	ci := &runnableComponent{
		name: name,
		state: ComponentState{
			Status: domain.StatusPending,
		},
		cfg: RunnableComponentConfig{
			Delay: time.Second,
		},
		waitErr: make(chan error, 10),
	}
	o.components[componentID] = ci
	return ci
}

func (o *Registry) NewComponent(componentID uint64, name string) Component {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, exists := o.components[componentID]; exists {
		panic("only one component per ID is allowed in the example app")
	}
	c := &component{name: name}
	o.components[componentID] = c
	return c
}

func (o *Registry) Get(componentID uint64) Component {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.components[componentID]
}

func (o *Registry) GetAll() map[uint64]Component {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.components
}
