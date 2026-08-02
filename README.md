# 🖼 Photo Frame App

Smart picture frame application designed to run on low-power hardware like a
Raspbery Pi Zero 2W.

When the application starts, it will run in fullscreen and expose a TikTok-like navigation
interface where the left / right halves of the screen are touch areas for going
backward / forward in the images.

## Prerequisites

For development, all prerequisites are included in the `devShell`'s in the `flake.nix`.

### Target Device

```bash
sudo apt update
sudo apt install libgl1-mesa-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev libxxf86vm-dev unclutter
```

Add the `frame-go.service` and `unclutter.service` unit files from the
`services/` directory in this repository to your target system and don't forget
to open these files and adjust any variables that may need adjusting, like
environment variables and file paths. The first service is meant to start the
`frame-go` binary itself, and `unclutter` is an application designed to hide the
mouse cursor in fullscreen applications.

### Environment Variables

```
S3_BUCKET_NAME=
AWS_ACCESS_KEY=
AWS_SECRET_KEY=
AWS_SESSION_TOKEN=
```

## Develop

```bash
nix develop
make build-native
```

## Build

For ARM64 target devices

```bash
make dev-arm
make build-arm
```

For local dev environments

```bash
make dev-native
make build-native
```

## Deploy

To deploy this application, simply copy the correct binary over to the target
device. Then we'll just need to copy over and activate the systemd unit file to
ensure it starts on boot. Don't forget to update the path to the binary in the
`frame-go.service` unit file (in `ExecStart`).

```bash
sudo cp frame-go.service /etc/systemd/system/frame-go.service
sudo systemctl daemon-reload
sudo systemctl enable --now frame-go
```

You should now have the fullscreen frame application running on the primary
display of the machine.

## License

MIT

