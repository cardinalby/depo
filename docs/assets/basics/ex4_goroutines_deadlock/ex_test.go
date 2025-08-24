package ex4_goroutines_deadlock

import (
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

func TestExample4GoroutineDeadlock(t *testing.T) {

	a := depo.Provide(func() *ComponentA {
		return NewComponentA("AA")
	})

	b := depo.Provide(func() *ComponentB {
		done := make(chan struct{})
		go func() {
			a() // a() is waiting for b() to finish providing to start a new providing session
			close(done)
		}()
		<-done // b() is waiting for goroutine to finish

		return NewComponentB("BB")
	})

	_ = b
	// uncomment to see the deadlock
	//b()
}
