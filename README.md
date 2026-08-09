# 🖼️ Domino Frame

Domino Frame is a fullscreen photo-frame application for low-power Linux devices, such as the Raspberry Pi Zero 2 W. Requires the [Domino Frame Web](https://github.com/ndom91/domino-frame-web) app to manage deployments.

## Features

- Displays a local photo collection in a fullscreen Fyne application.
- Supports touch and keyboard navigation. Tap the left or right half of the display, or use the arrow keys.
- Advances images automatically every 10 seconds.
- Syncs each frame's images from a Cloudflare R2 bucket every 10 minutes and keeps a local cache for offline use.
- Provides Bluetooth Low Energy setup only until the frame has successfully activated its metrics credential.
- Reports host uptime, filesystem capacity, and the currently displayed image to Domino Frame Web.

Images are stored under `images/<frame-id>` beside the executable. The frame ID, HTTPS API endpoint, and metrics credential are stored in `config.json` beside the executable. The file is written with mode `0600`.

## Documentation

- [Development](DEVELOPMENT.md): Nix development shells and native or ARM builds.
- [Deployment](DEPLOY.md): Raspberry Pi dependencies, configuration, and systemd services.

## License

MIT
