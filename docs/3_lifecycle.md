# Lifecycle-aware components

Some components are **"passive"**: they expose methods that can do some work including calling methods of their 
dependencies but don't actively perform any work by themselves if not called. Examples are services, use-cases, 
repositories, etc.

But there are also **"active"** (lifecycle-aware) components that: 
- listen for network requests (HTTP server, gRPC server)
- poll messages from the queue (Kafka consumer, RabbitMQ consumer)
- perform scheduled jobs (cron jobs)
- manage connections (database connection pool, cache connection pool) and need to flush buffers on shutdown
- monitor the file system (file watcher)

They can also depend on each other (directly or indirectly through passive components) and need to 
be **started** and **shut down** in the right **order**.

## Lifecycle phases

<p align="center">
    <img align="center" src="assets/lifecycle_types/phases.svg" alt="lifecycle phases"/>
</p>

The library supports different semantics to implement these phases:

<p align="center">
    <img align="center" src="assets/lifecycle_types/phase_interfaces.svg" alt="lifecycle phases"/>
</p>

Lifecycle behavior that is attached to a component is called a **lifecycle hook** in depo.

You can use different methods for different hooks, but **shouldn't mix** 
`Runnable` / `ReadinessRunnable` with `Starter` + `Closer` in the **same hook**.

## Separate phases semantics

Consists of two interfaces: `Starter` and `Closer`. Their methods are not async, but this semantics excludes
"Wait" phase (suitable for components that can't fail after being started).

### 🔹 Starter

```go
type Starter interface {
    Start(ctx context.Context) error
}
```

A Starter is a component that should be successfully started before dependent components can Start.
Examples are:
- DB migrations
- One-time initialization tasks (check DB schema, Kafka topics configs, etc.)
- Configuration providers
- Connections to DBs/queues that need to be established (normally have corresponding **Closer**)

If it's a **Closer**, **Close** will be called only if **Start** was successful

**Start()** should:
- block until the component is ready to be used and its dependents can **Start**
- return a **non-nil** error if the component **can't be started** (ctx.Err() if ctx is done)

### 🔹 Closer

```go
type Closer interface {
    Close()
}
```

A Closer is a component that should be stopped gracefully before stopping its dependencies during shutdown.
Examples are:
- Buffering message queue producers/caches/repos that should flush their buffers
- Connection pools with lazy connections (sql.DB)
- Externally started timers, opened files, etc.

If it's a **Starter**, **Close** will be called only if **Start** was successful

**Close** should **block** until the component is stopped.

## Waiting (Runnable) semantics

In this approach you lifecycle hook consists of a single `Run` method or function.

### 🔹 ReadinessRunnable

```go
type ReadinessRunnable interface {
    Run(ctx context.Context, onReady func()) error
}
```

It's an alternative to **Starter** + **Closer** + **Wait** health tracking. 

It notifies its **readiness** (when it has been started and doesn't block starts of dependents anymore) 
via `onReady` callback.

Run should:
- **Block** until the component **completes** its job (**or** is **stopped** via `ctx` cancellation).
- Call `onReady` callback once the component is **ready** and its **dependents can Start**
- Return **non-nil** error if the component **fails** and needs to trigger **shutdown**
- Shut down **gracefully** and return `ctx.Err()` if `ctx` is **canceled**. `context.Cause(ctx)` can be used to
  determine the cause of the shutdown.
- Return **nil** error if the component finishes successfully and **doesn't** want to trigger **shutdown**
  of other components. This default can be overridden by `OptNilRunResultAsError` option

### 🔹 Runnable

```go
type Runnable interface {
    Run(ctx context.Context) error
}
```

It's an alternative to "immediately ready **Starter**" + **Closer** + **Wait** health tracking.

It's the same as **ReadinessRunnable** except the lack of `onReady` callback.

Once **Run** is called, the component **immediately** unblocks dependents' starts (considered immediately ready).

## Lifecycle Hooks

To add lifecycle behavior to a component you can use 
[`depo.UseLifecycle()`](https://pkg.go.dev/github.com/cardinalby/depo#UseLifecycle) inside its `provide` function.

This way we can add and set up a "lifecycle hook" for the component

### 🔹 `AddStarter`

```go
a := depo.Provide(func() *ComponentA {
    componentA := NewComponentA("AA")
    // componentA should implement Starter
    depo.UseLifecycle().AddStarter(componentA)
    return componentA
})
```

> [!TIP]
> There is also `AddStartFn()` method that accepts just a `Start` function for the cases the component itself
> doesn't implement `Starter` interface

After adding `Starter` you can chain `AddCloser` / `AddCloseFn` call to define the shutdown behavior for this hook:

```go
depo.UseLifecycle().
    AddStarter(componentA)
    AddCloser(componentA) // componentA should implement Closer
```

<details>

<summary>✔️ OptStartTimeout(...)</summary>

You can pass optional `OptStartTimeout(timeout time.Duration)` argument to `AddStarter` / `AddStartFn`
to make [`Runner`](4_runner.md) use timeout context when calling `Start` method

The same option can be applied to all `Starter` / `ReadinessRunnable` components if passed to `NewRunner()`

</details>

### 🔹 `AddCloser`

```go
a := depo.Provide(func() *ComponentA {
    componentA := NewComponentA("AA")
    // componentA should implement Closer
    depo.UseLifecycle().AddCloser(componentA)
    return componentA
})
```

> [!TIP]
> There is also `AddCloseFn()` method that accepts just a `Close` function for the cases the component itself
> doesn't implement `Closer` interface

`AddCloser` / `AddCloseFn` can be used alone or chained with `AddStarter` / `AddStartFn`

### 🔹 `AddRunnable`

```go
a := depo.Provide(func() *ComponentA {
    componentA := NewComponentA("AA")
    // componentA should implement Runnable
    depo.UseLifecycle().AddRunnable(componentA)
    return componentA
})
```

> [!TIP]
> There is also `AddRunFn()` method that accepts just a `Run` function for the cases the component itself
> doesn't implement `Runnable` interface

`AddRunnable` / `AddRunFn` can't be chained with other methods that add lifecycle behavior.

<details>
<summary>✔️ OptNilRunResultAsError()</summary>

You can pass optional `OptNilRunResultAsError()` argument to `AddRunnable` / `AddRunFn` to treat `nil` result of 
`Run` method as an error that will trigger shutdown of other components with `ErrUnexpectedRunNilRunResult` cause.

The same option can be applied to all `Runnable` / `ReadinessRunnable` components if passed to `NewRunner()`
</details>

### 🔹 `AddReadinessRunnable`

```go
a := depo.Provide(func() *ComponentA {
    componentA := NewComponentA("AA")
    // componentA should implement ReadinessRunnable
    depo.UseLifecycle().AddReadinessRunnable(componentA)
    return componentA
})
```

> [!TIP]
> There is also `AddReadinessRunFn()` method that accepts just a `Run` function for the cases the component itself
> doesn't implement `ReadinessRunnable` interface  

`AddReadinessRunnable` / `AddReadinessRunFn` can't be chained with other methods that add lifecycle behavior

<details>
<summary>✔️ OptStartTimeout(...)</summary>

You can pass optional `OptStartTimeout(timeout time.Duration)` argument to `AddReadinessRunnable` / `AddReadinessRunFn`
to make [`Runner`](4_runner.md) cancel run context after the timeout if `onReady` callback hasn't been called yet.

The same option can be applied to all `Starter` / `ReadinessRunnable` components if passed to `NewRunner()`

</details>

<details>
<summary>✔️ OptNilRunResultAsError()</summary>

You can pass optional `OptNilRunResultAsError()` argument to `AddReadinessRunnable` / `AddReadinessRunFn` to 
treat `nil` result of `Run` method as an error that will trigger shutdown of other components.

The same option can be applied to all `Runnable` / `ReadinessRunnable` components if passed to `NewRunner()`
</details>

## Multiple Lifecycle Hooks

Even though that is not normally needed, you can call `depo.UseLifecycle()` multiple times, creating multiple lifecycle hooks 
for the same component.

They will act as independent instances with their own lifecycles that have the same set of dependencies and dependents
(inherited from the component).

You can also use an arbitrary **tag** to distinguish between multiple hooks of the same component when observing
lifecycle events via [`RunnerListener`](https://pkg.go.dev/github.com/cardinalby/depo#RunnerListener) calls and
in [`ErrLifecycleHookFailed`](https://pkg.go.dev/github.com/cardinalby/depo#ErrLifecycleHookFailed) error.

```go
a := depo.Provide(func() *ComponentA {
    componentA := NewComponentA("AA")
    depo.UseLifecycle().AddCloser(func () { ... }).Tag("tag1")
    depo.UseLifecycle().AddCloser(func () { ... }).Tag("tag2")
    return componentA
})
```

## The last step

👉 [Application Runner](4_runner.md)
