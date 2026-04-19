# Runners

Runners is a lightweight, Go-based CLI utility designed to manage multiple GitHub Actions self-hosted runners on a single host using Docker. It provides environment isolation, resource constraints, and native support for Docker-in-Docker (DinD) workflows.

## Features

- Docker-based Isolation: Each runner operates within its own containerized environment.
- Resource Constraints: Define hard limits for CPU (cores) and Memory (MB) per runner.
- Dynamic Updates: Modify CPU and Memory limits for running containers using the Docker Container Update API without downtime.
- Docker-in-Docker (DinD): Native support for running Docker commands within GitHub Actions by mounting the host Docker socket.
- Monitoring: Real-time tracking of runner status, uptime, and cumulative error counts.
- Mass Operations: Atomic-like stop and reboot commands for all configured runners, processed in alphabetical order.
- Persistent Configuration: Local JSON-based configuration management.

## Prerequisites

- Go 1.25 or higher
- Docker Engine installed and running
- Sufficient privileges to interact with the Docker daemon (e.g., membership in the `docker` group on Linux)

## Installation

### Linux

1. Clone the repository.
2. Execute the installation script:
   ```bash
   chmod +x install.sh
   ./install.sh
   ```
This compiles the binary and moves it to `/usr/local/bin/runners`.

### Windows

1. Ensure Go is installed.
2. Build the executable:
   ```powershell
   go build -o runners.exe
   ```

## Usage

### Commands Overview

| Command | Description | Supports `--all` / `-a` |
| :--- | :--- | :---: |
| `add` | Register and start a new GitHub runner | No |
| `list` | List all runners with status, uptime, and limits | N/A |
| `start` | Start one or all stopped runners | Yes |
| `stop` | Stop one or all running runners | Yes |
| `reboot` | Restart one or all runners | Yes |
| `update` | Update resource limits (CPU/RAM) dynamically | Yes |
| `remove` | Stop, remove container, and delete configuration | Yes |

### Examples

#### Registering a new Runner
Requires a registration token from GitHub (Settings -> Actions -> Runners).
```bash
runners add --name "runner-01" --url "https://github.com/org/repo" --token "TOKEN" --cpu 1.0 --memory 2048
```

#### Monitoring
```bash
runners list
```

#### Resource Management
Update limits for a specific runner or all of them. Changes apply immediately to running containers.
```bash
# Update single runner
runners update "runner-01" --cpu 2.0 --memory 4096

# Update all runners at once
runners update --all --cpu 0.5 --memory 1024
```

#### Lifecycle Control
All mass operations (`--all`) are processed in alphabetical order.
```bash
# Start all runners
runners start -a

# Stop all runners
runners stop -a

# Reboot a specific runner
runners reboot "runner-01"

# Wipe everything
runners remove --all
```
## Technical Architecture

- Runner Image: Utilizes `myoung34/github-runner:latest`.
- Privileged Mode: Containers run in `--privileged` mode to support DinD and advanced resource management.
- Host Integration: Mounts `/var/run/docker.sock` to provide the runner with access to the host's Docker engine.
- Configuration: State is persisted in `~/.runners/config.json`.
- Process Ordering: Mass operations follow the same lexicographical sorting as the `list` command for deterministic behavior.

## Uninstallation (Linux)

To remove the binary and stop all associated containers:
```bash
chmod +x uninstall.sh
./uninstall.sh
```
