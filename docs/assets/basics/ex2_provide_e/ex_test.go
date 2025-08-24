package ex2_provide_e

import (
	"errors"
	"fmt"
	"log"
	"testing"

	"github.com/cardinalby/depo"
)

type ComponentA struct {
	Name string
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

func NewComponentC(
	name string,
	a *ComponentA,
	b *ComponentB,
) *ComponentC {
	return &ComponentC{
		Name: name,
		A:    a,
		B:    b,
	}
}

type ComponentD struct {
	Name string
	A    *ComponentA
	B    *ComponentB
}

func NewComponentD(
	name string,
	a *ComponentA,
	b *ComponentB,
) *ComponentD {
	if b == nil {
		panic("b is required")
	}
	return &ComponentD{
		Name: name,
		A:    a,
		B:    b,
	}
}

func TestExample1Abc(t *testing.T) {
	// `a` is a component that can fail to create
	a := depo.ProvideE(func() (*ComponentA, error) {
		return nil, errors.New("error creating A")
	})

	b := depo.Provide(func() *ComponentB {
		return NewComponentB("BB")
	})

	c := depo.ProvideE(func() (*ComponentC, error) {
		compA, err := a()
		if err != nil {
			return nil, fmt.Errorf("error getting component A: %w", err)
		}
		return NewComponentC(
			"CC",
			compA,
			b(),
		), nil
	})

	d := depo.Provide(func() *ComponentD {
		compA, err := a()
		if err != nil {
			log.Printf("`d` will do without `a`: %v", err)
		}
		return NewComponentD(
			"DD",
			compA,
			b(),
		)
	})

	fmt.Println(a()) // <nil> error creating A
	fmt.Println(c()) // <nil> error getting component A: error creating A
	fmt.Println(d()) // &{DD <nil> <b_ptr>} <--- created without A
}
