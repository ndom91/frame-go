Photo Frame App


## Prerequisites

### On the Pi

```
sudo apt update
sudo apt install libgl1-mesa-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev libxxf86vm-dev
```

## Development

```
nix develop
make build-native
```

## Building

For arm:
```
make build-arm
```

For dev-environment native:
```
make build-native
```

## License

MIT

