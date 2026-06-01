# locdaemon

Generic local JSON-RPC daemon kit: home layout paths, single-instance lock, accept loop, observe HTTP, and client dial/spawn helpers.

## Install

From [pkg.go.dev](https://pkg.go.dev/github.com/brandonkramer/locdaemon):

```bash
go get github.com/brandonkramer/locdaemon
```

## Quick start

```go
ctx := context.Background()

err := runtime.Run(ctx, runtime.Config{
    Home:     home,
    Layout:   layout,
    Assigner: myRPCAssigner,
    Prepare:  prepareHome,
    OnReady:  startBackgroundWork,
})
if err != nil {
    return err
}
```

Client side:

```go
got, err := client.CallHome[string](ctx, home, layout, "status", nil, 5*time.Second)
if err != nil {
    return err
}
```

## Development

Lefthook and golangci-lint are pinned in `go.mod` as **tools** (dev-only). Install git hooks once per clone:

```bash
make install-hooks
```

```bash
make check
make test
make lint
```
