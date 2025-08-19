"depo" is a complete solution for managing **dependency construction** and **lifecycles** in Golang projects.

## Alternative to automated Dependency Injection

Automated DI frameworks ([Uber Fx](https://github.com/uber-go/fx) or [Wire](https://github.com/google/wire)) 
don't play well with the absence of annotations and the "local interfaces" idea in Go.

With this library you still manually choose the dependencies to initialize your components but also benefit
from traditional DI features like:
- Building a **dependency graph** to use for running/shutting down components in the right order
- **Lazy initialization** of components (avoid creating components that are never used)
- Resolving **circular dependencies** (that's what both Uber Fx and Wire failed to do but is handled in other languages)
  by using **late initialization**

## Concurrent run and shutdown ([demo](https://cardinalby.github.io/depo/))

Since the library builds the dependency graph, it can [Start/Close/Run](lifecycle_types.go) components in the proper order
**concurrently** when possible (unlike Uber Fx, which does it sequentially), speeding up your starts and shutdowns. 

The library supports assigning separate, well-thought-out lifecycle roles to components and managing the entire 
application lifecycle using `SIGINT/SIGTERM` shutdown context.

## Composable design

The root `Runner` object (that is supposed to run other components) implements [`ReadinessRunnable`](lifecycle_types.go) 
itself, allowing you to use it just as any other component
