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

func TestExample3UsePtrLateInit(t *testing.T) {
	var components struct {
		A func() *ComponentA
		B func() *ComponentB
	}

	components.A = depo.Provide(func() *ComponentA {
		return depo.UsePtrLateInit(func() *ComponentA {
			return &ComponentA{
				Name: "AA",
				B:    components.B(),
			}
		})
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
