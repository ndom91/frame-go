# 🖼 Photo Frame App

Smart picture frame application designed to run on low-power hardware like a
Raspbery Pi Zero 2W.

When the application starts, it will run in fullscreen and expose a TikTok-like navigation
interface where the left / right halves of the screen are touch areas for going
backward / forward in the images.

Todos:

- [ ] Implement S3 fetching of images instead of just local filesystem-based ones
- [ ] Implement BLE GATT server so picture frames can be easily initialised by users in the
  [frame-web](https://github.com/ndom91/frame-web) admin portal.

## Prerequisites

For development, all prerequisites are included in the `devShell`'s in the `flake.nix`.

### Target Device

```bash
sudo apt update
sudo apt install libgl1-mesa-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev libxxf86vm-dev
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

