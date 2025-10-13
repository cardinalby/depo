# Example multi-cmd project

This is an example of a project with multiple executables sharing the same codebase.

It's common in "modular monolith" architectures and demonstrates how lazy `depo` providers can be used
in this case.

The following diagram shows **all** the components of the project:

<p align="center">
    <img align="center" src="docs/all_components.svg" alt="all components"/>
</p>

The yellow components are "active": they have lifecycle hooks defined in their `provide` functions.
The project consists of 2 applications:

## 1. api_server

It runs an HTTP server exposing a REST API for adding/reading cats. It also logs requests to a file.
The diagram below shows the components that are getting created when `api_server` is run:

<p align="center">
    <img align="center" src="docs/api_server_components.svg" alt="api server components"/>
</p>

### How to run 
```bash
go run ./cmd/api_server
```

Example requests to API server:

```
POST http://localhost:8080/cats

{
    "name": "Mittens",
    "age": 3
}
```

```
GET http://localhost:8080/cats
```

## 2. cli_history_exporter

It's a CLI tool that prints the requests history from `LogFile` to the console in xml or json format.

Only a subset of components is lazily created for this application.

<p align="center">
    <img align="center" src="docs/cli_history_exporter_components.svg" alt="cli exporter components"/>
</p>

### How to run

```bash
go run ./cmd/cli_history_exporter
```

