# kv

View Kubernetes resources in a web UI. `kv` reads a list of context/namespace
pairs from a config file and serves a dashboard of their pods, deployments,
services, ingresses, stateful sets, daemon sets, jobs, and cron jobs — with
pod logs, live metrics, YAML describe, and delete.

## Requirements

- A working kubeconfig. Set `KUBECONFIG`, pass `--kubeconfig`, or place a config
  at `~/.kube/config`.
- The cluster's metrics server, if you want pod CPU/memory (optional; other
  resources work without it).

## Configuration

`kv` reads a `kv.context` file (default `~/kv.context`) listing which
context/namespace pairs to show, one per line:

```
<kube-context>:<namespace>
```

- Lines starting with `#` are comments; blank lines are ignored.
- The context must exist in your kubeconfig.
- Omit the namespace (just the context) to list resources across all namespaces.

Copy the example and edit it with real values:

```bash
cp kv.context.example ~/kv.context
```

See [`kv.context.example`](kv.context.example) for a full sample.

## Usage

```
Usage: kv [flags]

View Kubernetes resources in UI.

Flags:
  -h, --help                     Show context-sensitive help.
  -k, --kubeconfig=STRING        Path to kubeconfig file ($KUBECONFIG).
  -c, --config="~/kv.context"    Path to kv.context file.
  -a, --addr=":8989"             Address to listen on.
```

The server listens on `http://localhost:8989` by default.

## Development

```bash
# Install dependencies
make install

# Run with hot reload (recommended)
make dev

# Build and run normally
make run

# Build the binary to bin/kv (runs lint first)
make build

# Lint only
make lint
```

`make build` runs `golangci-lint` as a prerequisite, so a lint failure fails the
build. Linting requires a `golangci-lint` built with the module's Go toolchain;
install it with `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`.
