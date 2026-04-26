# Runners

Runners is a lightweight, Go-based CLI utility designed to manage multiple GitHub Actions self-hosted runners on a single host using Docker. It provides environment isolation, resource constraints, and native support for Docker-in-Docker (DinD) workflows.

## Features

- Docker-based Isolation: Each runner operates within its own containerized environment.
- Resource Constraints: Define hard limits for CPU (cores) and Memory (MB) per runner.
- Dynamic Updates: Modify CPU and Memory limits for running containers using the Docker Container Update API without downtime.
- Docker-in-Docker (DinD): Native support for running Docker commands within GitHub Actions by mounting the host Docker socket.
- Monitoring: Real-time tracking of runner status, uptime, and cumulative error counts.
- Bulk Operations: `start`, `stop`, `reboot`, `update`, and `remove` accept any list of runner names or `--all` for the whole fleet.
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

| Command | Description | Multi-target |
| :--- | :--- | :--- |
| `add` | Register and start a new GitHub runner | No |
| `list` | List all runners with status, uptime, and limits | N/A |
| `log` | Print a log from runner's container (Docker syntax) | N/A |
| `start` | Start one, several, or all stopped runners | `name...` or `--all` |
| `stop` | Stop one, several, or all running runners | `name...` or `--all` |
| `reboot` | Restart one, several, or all runners (alias: `restart`) | `name...` or `--all` |
| `update` | Update resource limits (CPU/RAM) dynamically | `name...` or `--all` |
| `remove` | Stop, remove container, and delete configuration | `name...` or `--all` |

Multi-target commands accept any number of runner names. Unknown names are skipped with a warning and the remaining runners are still processed. `--all` (`-a`) ignores positional args and applies to every configured runner in alphabetical order.

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
Update limits for one, several, or all runners. Changes apply immediately to running containers.
```bash
# Single runner
runners update "runner-01" --cpu 2.0 --memory 4096

# Several runners at once
runners update "runner-01" "runner-02" --cpu 1.0 --memory 2048

# All runners
runners update --all --cpu 0.5 --memory 1024
```

#### Lifecycle Control
Pass any number of runner names, or use `--all` (alphabetical order). Unknown names are skipped without aborting the rest.
```bash
# Start a few specific runners
runners start "runner-01" "runner-02"

# Stop several runners
runners stop "runner-01" "runner-03" "runner-05"

# Reboot all runners
runners reboot -a

# `restart` is an alias for `reboot`
runners restart "runner-01" "runner-02"

# Wipe everything
runners remove --all
```
## Technical Architecture

- Runner Image: Utilizes `myoung34/github-runner:latest`.
- Privileged Mode: Containers run in `--privileged` mode to support DinD and advanced resource management.
- Host Integration: Mounts `/var/run/docker.sock` to provide the runner with access to the host's Docker engine.
- Configuration: State is persisted in `~/.runners/config.json`.
- Process Ordering: `--all` operations follow the same lexicographical sorting as the `list` command for deterministic behavior. Multi-name invocations preserve the order given on the command line.
- Auto-Update Resilience: Containers are launched with `./run.sh` as the entrypoint command, so the upstream actions/runner update loop (exit code 3 → apply staged update → relaunch listener) runs **inside** a single container instead of crashing it. `start` / `reboot` automatically recreate containers that exited with a non-zero code, so a single `reboot --all` heals runners broken by a botched self-update.

## Uninstallation (Linux)

To remove the binary and stop all associated containers:
```bash
chmod +x uninstall.sh
./uninstall.sh
```
