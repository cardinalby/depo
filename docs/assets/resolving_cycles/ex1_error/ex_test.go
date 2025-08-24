package ex1_error

import (
	"testing"

	"github.com/cardinalby/depo"
)

type ComponentA struct {
	Name string
	B    *ComponentB
}

func NewComponentA(name string, b *ComponentB) *ComponentA {
	return &ComponentA{
		Name: name,
		B:    b,
	}
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

func TestExample1Cyclic(t *testing.T) {
	var components struct {
		A func() *ComponentA
		B func() *ComponentB
	}

	components.A = depo.Provide(func() *ComponentA {
		return NewComponentA(
			"AA",
			components.B(), // A depends on B
		)
	})

	components.B = depo.Provide(func() *ComponentB {
		return NewComponentB(
			"BB",
			components.A(), // B depends on A
		)
	})

	// panic!
	// components.A()
}
