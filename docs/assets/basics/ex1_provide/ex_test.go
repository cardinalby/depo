package ex1_provide

import (
	"fmt"
	"testing"

	"github.com/cardinalby/depo"
)

type ComponentA struct {
	Name string
}

func NewComponentA(name string) *ComponentA {
	return &ComponentA{Name: name}
}

type ComponentB struct {
	Name string
}

func NewComponentB(name string) *ComponentB {
	return &ComponentB{Name: name}
}

type ComponentC struct {
	Name string
	A    *ComponentA
	B    *ComponentB
}

func NewComponentC(name string, a *ComponentA, b *ComponentB) *ComponentC {
	return &ComponentC{
		Name: name,
		A:    a,
		B:    b,
	}
}

func TestExample1Abc(t *testing.T) {
	// componentA is `func() *ComponentA`
	a := depo.Provide(func() *ComponentA {
		return NewComponentA("AA") // has no dependencies
	})

	b := depo.Provide(func() *ComponentB {
		return NewComponentB("BB") // has no dependencies
	})

	c := depo.Provide(func() *ComponentC {
		return NewComponentC(
			"CC", // componentC depends on:
			a(),  // - componentA
			b(),  // - componentB
		)
	})

	// CC AA BB
	fmt.Println(
		c().Name,
		a().Name,
		b().Name,
	)
}
