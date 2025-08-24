package ex1_use_component_id

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

func TestExampleDebugHooksComponentId(t *testing.T) {
	a := depo.Provide(func() *ComponentA {
		fmt.Println(depo.UseComponentID()) // 1
		return NewComponentA("AA")
	})

	b := depo.Provide(func() *ComponentB {
		fmt.Println(depo.UseComponentID()) // 2
		return NewComponentB("BB")
	})

	a()
	b()
}
