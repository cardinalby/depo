# Debug info

The library provides some observability utilities for the cases when you need to debug or log the dependency graph.

> To observe `Runner` behavior, use the `OptRunnerListeners()` option passed to `NewRunner()`

## 🔹 [`UseComponentID() uint64`](https://pkg.go.dev/github.com/cardinalby/depo#UseComponentID)

Each component has an internal unique `uint64` ID that can be observed in
[`RunnerListener`](https://pkg.go.dev/github.com/cardinalby/depo#RunnerListener) calls and 
in [`ErrLifecycleHookFailed`](https://pkg.go.dev/github.com/cardinalby/depo#ErrLifecycleHookFailed) error.

Being inside a [`provide`](1_basics.md) function, you can obtain it by calling 
`depo.UseComponentID()` ([EX1](assets/debug_info/ex_test.go)):

```go
a := depo.Provide(func() *ComponentA {
    fmt.Println(depo.UseComponentID()) // 1
    return NewComponentA("AA")
})

b := depo.Provide(func() *ComponentB {
    fmt.Println(depo.UseComponentID()) // 2
    return NewComponentB("BB")
})
```

## 🔹 [`UseTag(tag any)`](https://pkg.go.dev/github.com/cardinalby/depo#UseTag)

Calling this hook inside a [`provide`](1_basics.md) function, you can assign an arbitrary tag to the component 
that will be observed later in the 
[`RunnerListener`](https://pkg.go.dev/github.com/cardinalby/depo#RunnerListener) calls and in
[`ErrLifecycleHookFailed`](https://pkg.go.dev/github.com/cardinalby/depo#ErrLifecycleHookFailed) error
as [`LifecycleHookInfo.ComponentInfo().Tag()`](https://pkg.go.dev/github.com/cardinalby/depo#ComponentInfo)  
([EX2](assets/debug_info/ex2_use_tag/ex2_test.go)):

```go
a := depo.Provide(func() *ComponentA {
    depo.UseTag("tagA")
    return NewComponentA("AA")
})
```

## 🔹 [`Runner.GetRootLifecycleHookNodes()`](https://pkg.go.dev/github.com/cardinalby/depo#Runner)

The method returns a slice of root 
[`LifecycleHookNode`](https://pkg.go.dev/github.com/cardinalby/depo#LifecycleHookNode) items.

```go
type LifecycleHookNode interface {
    LifecycleHookInfo
    DependsOnHooks() []LifecycleHookNode
}
```

Traversing the graph (that can't have cycles), you can obtain the entire dependency graph of Lifecycle hooks that
are going to be executed by the `Runner`. It contains information only about components with lifecycle hooks.