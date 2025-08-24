package ex2_use_late_init

import (
	"fmt"
	"testing"

	"github.com/cardinalby/depo"
)

type ComponentA struct {
	Name string
	B    *ComponentB
}

func NewComponentA(name string) *ComponentA {
	return &ComponentA{
		Name: name,
	}
}

func (a *ComponentA) SetB(b *ComponentB) {
	a.B = b
}

type ComponentB struct {
	Name string
	A    *ComponentA
}

func NewComponentB(name string, a *ComponentA) *ComponentB {
	return &ComponentB{
		Name: name,
		A:    a,
	}
}

func TestExample2UseLateInit(t *testing.T) {
	var components struct {
		A func() *ComponentA
		B func() *ComponentB
	}

	components.A = depo.Provide(func() *ComponentA {
		a := NewComponentA("AA")
		depo.UseLateInit(func() {
			// Late-init callback gets executed before resolving chain is completed.
			// At this point B is fully initialized,
			// and we can use it to finish A initialization
			a.SetB(components.B())
		})
		// Late-init callback will be executed later.
		// At this point A doesn't depend on B yet
		return a
	})

	components.B = depo.Provide(func() *ComponentB {
		return NewComponentB(
			"BB",
			components.A(), // it returns partially initialized A (without B yet)
		)
	})

	fmt.Println(
		components.A().Name,   // AA
		components.A().B.Name, // BB
		components.B().Name,   // BB
		components.B().A.Name, // AA
	)
}
