# Domino Frame

Domino Frame is a fullscreen photo-frame application for low-power Linux devices, including the Raspberry Pi Zero 2 W.

## Features

- Displays a local photo collection in a fullscreen Fyne application.
- Supports touch and keyboard navigation. Tap the left or right half of the display, or use the arrow keys.
- Advances images automatically every 10 seconds.
- Syncs each frame's images from a Cloudflare R2 bucket every 10 minutes and keeps a local cache for offline use.
- Provides a Bluetooth Low Energy setup service for Wi-Fi credentials and frame configuration.

Images are stored under `images/<frame-id>` beside the executable. The frame ID is generated on first start and kept in `config.json`.

## Documentation

- [Development](DEVELOPMENT.md): Nix development shells and native or ARM builds.
- [Deployment](DEPLOY.md): Raspberry Pi dependencies, configuration, and systemd services.

## License

MIT
