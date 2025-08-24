package ex3_goroutine_valid

import (
	"sync"
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
	A    *ComponentA
}

func NewComponentB(name string, a *ComponentA) *ComponentB {
	return &ComponentB{Name: name, A: a}
}

func TestExample4GoroutineDeadlock(t *testing.T) {
	a := depo.Provide(func() *ComponentA {
		return NewComponentA("AA")
	})

	b := depo.Provide(func() *ComponentB {
		return NewComponentB("BB", a())
	})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		a() // waits until providing is done in second goroutine (if it started first)
		wg.Done()
	}()

	go func() {
		b() // waits until providing is done in first goroutine (if it started first)
		wg.Done()
	}()

	wg.Wait()
}
