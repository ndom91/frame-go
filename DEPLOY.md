# Deployment

Deploy Domino Frame to a Raspberry Pi Zero 2 W or another ARM64 Linux device running a graphical X session.

## Target dependencies

Install the graphics libraries and `unclutter` on the device:

```bash
sudo apt update
sudo apt install libgl1-mesa-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev libxxf86vm-dev unclutter
```

The Bluetooth setup service connects to Wi-Fi with `nmcli`, so NetworkManager must be installed and managing the Wi-Fi adapter.

## Build and copy

On macOS, build with Docker and copy the binary:

```bash
make build-arm
scp domino-frame-arm pi@<host>:/home/pi/domino-frame
```

On Linux with Nix, use the existing cross-compilation target:

```bash
nix develop .#arm
make build-arm
scp domino-frame-arm pi@<host>:/home/pi/domino-frame
```

On Linux, `make deploy` builds and copies the binary. Override `PI_HOST` and `PI_TARGET_DIR` as needed. That target currently uses `~/.ssh/id_ndo4`.

## Configure the service

Review `services/domino-frame.service` before installation. Set `User`, `WorkingDirectory`, `DISPLAY`, `ExecStart`, and these Cloudflare R2 values for the target:

```ini
Environment=R2_BUCKET_NAME=your-bucket-name
Environment=R2_ACCESS_KEY=access-key
Environment=R2_SECRET_KEY=secret-key
Environment=CF_ACCOUNT_ID=cloudflare-account-id
```

Install the application service and the cursor-hiding service:

```bash
sudo cp services/domino-frame.service /etc/systemd/system/domino-frame.service
sudo cp services/unclutter.service /etc/systemd/system/unclutter.service
sudo systemctl daemon-reload
sudo systemctl enable --now domino-frame unclutter
```

Check the application with:

```bash
systemctl status domino-frame
journalctl -u domino-frame -f
```
