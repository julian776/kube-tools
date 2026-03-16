# kube-tools

A CLI toolkit for visualizing Kubernetes resource usage.

## Features

- **Interactive graph view** with time range tabs (1 Hour, 4 Hours, 1 Day, Today)
- CPU and memory usage bar charts per container
- Pod and deployment level metrics
- Dynamic shell autocompletion (pod/deployment names from your cluster)

## Installation

```bash
go install github.com/julianalvarez/kube-tools@latest
```

Or build from source:

```bash
git clone https://github.com/julianalvarez/kube-tools.git
cd kube-tools
go build -o kube-tools .
```

## Prerequisites

- A Kubernetes cluster with [metrics-server](https://github.com/kubernetes-sigs/metrics-server) installed
- `kubectl` configured with a valid kubeconfig

## Usage

### Graph pod resource usage

```bash
kube-tools graph pod <pod-name>
```

### Graph deployment resource usage

```bash
kube-tools graph deployment <deployment-name>
```

### Options

```bash
# Specify namespace
kube-tools graph pod <pod-name> -n kube-system

# Specify kube context
kube-tools graph pod <pod-name> --context my-cluster
```

### Interactive controls

| Key | Action |
|---|---|
| `←` / `→` / `h` / `l` | Switch time range tab |
| `Tab` / `Shift+Tab` | Switch time range tab |
| `q` / `Esc` | Quit |

## Shell completion

```bash
# Bash
source <(kube-tools completion bash)

# Zsh
source <(kube-tools completion zsh)

# Fish
kube-tools completion fish | source

# PowerShell
kube-tools completion powershell | Out-String | Invoke-Expression
```

Add the appropriate line to your shell profile for persistent completion.

## Running tests

```bash
go test ./...
```

## Project structure

```
├── main.go                      # Entrypoint
├── cmd/
│   ├── root.go                  # Root command, --context flag
│   ├── graph.go                 # graph subcommand, -n flag
│   ├── graph_pod.go             # graph pod <name>
│   └── graph_deployment.go      # graph deployment <name>
├── pkg/
│   ├── kube/
│   │   └── client.go            # Kubernetes + Metrics API client
│   └── graph/
│       ├── render.go            # Terminal bar chart renderer
│       └── tui.go               # Interactive TUI with time range tabs
```
