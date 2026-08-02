# Development

## Prerequisites

Nix provides the Go toolchain and native graphics dependencies for Linux through `flake.nix`.

## macOS

Use Docker and Fyne's `fyne-cross` tool to cross-compile the ARM64 Linux binary for the frame. Install [Go](https://go.dev/dl/) and [Docker Desktop](https://www.docker.com/products/docker-desktop/), then ensure Docker is running.

Install `fyne-cross` once:

```bash
go install github.com/fyne-io/fyne-cross@latest
```

Build the Raspberry Pi binary:

```bash
make build-arm-docker
```

This produces `photo-frame-arm`. See [DEPLOY.md](DEPLOY.md) for the deployment steps.

Native macOS builds are not currently supported. The application uses Bluetooth Low Energy peripheral APIs that `tinygo.org/x/bluetooth` exposes on Linux but not macOS.

If the project has no `Icon.png`, `fyne-cross` creates a placeholder. Replace it with a project icon before releasing a package.

## Linux native development

Start the default development shell, then build or run the application:

```bash
nix develop
make build-native
./photo-frame
```

`make dev` builds and starts the native binary in one command.

## ARM64 build on Linux

Use the ARM development shell to cross-compile for a Raspberry Pi Zero 2 W:

```bash
nix develop .#arm
make build-arm
```

This produces `photo-frame-arm`. Use `make clean` to remove local binaries and Nix build output.

See [DEPLOY.md](DEPLOY.md) for the deployment steps.

## Useful variables

`make deploy` accepts these optional variables:

```bash
PI_HOST=pi@10.0.1.10
PI_TARGET_DIR=/home/pi
```

See [DEPLOY.md](DEPLOY.md) for target setup and service installation.
