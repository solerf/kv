```
Usage: kv [flags]

View Kubernetes resources in UI.

Flags:
  -h, --help                     Show context-sensitive help.
  -k, --kubeconfig=STRING        Path to kubeconfig file ($KUBECONFIG).
  -c, --config="~/kv.context"    Path to kv.context file.
  -a, --addr=":8989"             Address to listen on.
```

## Development

```bash
# Install dependencies
make install

# Run with hot reload (recommended)
make dev

# Build and run normally
make run
```
