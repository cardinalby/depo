package ex2_use_tag

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cardinalby/depo"
)

type ComponentA struct {
	Name string
}

func (a *ComponentA) Run(_ context.Context) error {
	return nil
}

func NewComponentA(name string) *ComponentA {
	return &ComponentA{Name: name}
}

func TestExampleDebugHooksComponentId(t *testing.T) {
	a := depo.Provide(func() *ComponentA {
		depo.UseTag("tagA")
		depo.UseLifecycle().AddStartFn(func(ctx context.Context) error {
			return errors.New("my_error")
		})
		return NewComponentA("AA")
	})

	runner := depo.NewRunner(func() {
		a()
	})
	err := runner.Run(context.Background(), nil)

	fmt.Println(err.Error())
	// Start of Starter registered at
	//     /depo/docs/assets/debug_hooks/ex2_use_tag/ex2_test.go:27
	// In Provide(1, tag: tagA) *ex2_use_tag.ComponentA @ /depo/docs/assets/debug_hooks/ex2_use_tag/ex2_test.go:25
	// failed: my_error

	var lcHookFailedErr depo.ErrLifecycleHookFailed
	if errors.As(err, &lcHookFailedErr) {
		fmt.Println(lcHookFailedErr.LifecycleHook().ComponentInfo().Tag())
		// tagA
	}
}
