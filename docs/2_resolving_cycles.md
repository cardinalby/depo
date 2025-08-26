# Resolving Cycles

## Unhandled cycle example

Let's add a cyclic dependency between `a` and `b` and see the result
([EX1](./assets/resolving_cycles/ex1_error/ex_test.go)):

<p align="center">
    <img align="center" src="assets/resolving_cycles/ex1_error/direct_cycle.svg" alt="example2 graph"/>
</p>

```go
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

// panic:
// cyclic dependency. Use UseLateInit() to solve
// —→ Provide(1) *resolving_cycles.ComponentA @ /example1_cyclic_test.go:40
//     /example1_cyclic_test.go:50
// In Provide(2) *resolving_cycles.ComponentB @ /example1_cyclic_test.go:47
//     /example1_cyclic_test.go:43
// In Provide(1) *resolving_cycles.ComponentA @ /example1_cyclic_test.go:40

components.A()
```

You get an `ErrCyclicDependency` error thrown in panic. The error contains details of the chain that caused the cycle.

> [!NOTE]
> Use `ProvideE` instead of `Provide` if you want to return the error instead of panicking

## Late Initialization to resolve cycles

### 🔹 `UseLateInit()` or `UseLateInitE()`
The common approach to resolve cycles is to use **pointers** and **late initialization** for at least one 
component in the chain ([EX2](./assets/resolving_cycles/ex2_use_late_init/ex_test.go)):

```go
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
```

This way we break the initialization into **2 phases**:
1. Creating components and linking them without cycles
2. Executing late-init callbacks to finish initialization. At this step we use already created (and memoized
   components and don't face cycles as well

Note that:
- It doesn't matter which component in the cycle uses 
  [`UseLateInit()`](https://pkg.go.dev/github.com/cardinalby/depo#UseLateInit)
  to break the cycle. The important part is that at least one component in the cycle should use it
- You can use both `components.A()` or `components.B()` as an entry point. The late-init callback will be
  executed after the 1st phase is done and before the entry-point provider returns

> [!NOTE]
> There is a [`UseLateInitE`](https://pkg.go.dev/github.com/cardinalby/depo#UseLateInitE) version of the function 
> if you want to return an explicit error from the late-init callback.
> 
> The error will fail the component itself and dependent components.
> Failed components will throw the wrapped error in panic if they are declared with `Provide` or return
> the error if they are declared with `ProvideE`

### 🔹 [`UsePtrLateInit()`](https://pkg.go.dev/github.com/cardinalby/depo#UsePtrLateInit)

It's a **helper** function that you can use to late-initialize pointer to a **copyable struct**
([EX3](./assets/resolving_cycles/ex3_use_ptr_late_init/ex_test.go)):

```go
components.A = depo.Provide(func() *ComponentA {
    // Helper function creates new(*ComponentA) and returns it
    // Also it registers late-init callback that creates another *ComponentA
    // and copies its value to the initial pointer 
    // (that is used by other components)
    return depo.UsePtrLateInit(func() *ComponentA {
        return &ComponentA{
            Name: "AA",
            B:    components.B(),
        }
    })
})
```

> [!CAUTION]
> It can only be used if your struct is safe to copy (has no mutexes, atomic values, etc...).

## Non-lifecycle aware components only

This technique can be used for components that don't have lifecycle hooks (use-cases/services/repositories, etc.).

In other words, it solves cycles in component initialization, but the cycles between lifecycle-aware components
prevent the `Runner` from finding out the proper order of starting/shutting down components

➡️ [Components' lifecycle](./3_lifecycle.md)
