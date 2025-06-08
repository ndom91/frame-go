{
  description = "Digital Photo Frame - Cross-compilation flake for ARM";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Cross-compilation target for ARM (Pi Zero 2W)
        armPkgs = pkgs.pkgsCross.armv7l-hf-multiplatform;

        # Build the photo frame binary for ARM
        photo-frame-arm = armPkgs.buildGoModule rec {
          pname = "photo-frame";
          version = "0.1.0";

          src = ./.;

          # You'll need to update this hash after first build
          # Run: nix build .#photo-frame-arm
          # Then use the hash from the error message
          vendorHash = null; # or "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

          # CGO is needed for BLE libraries
          CGO_ENABLED = "1";

          # Set target architecture
          GOOS = "linux";
          GOARCH = "arm";
          GOARM = "7";

          # Build flags for smaller binary
          ldflags = [ "-s" "-w" ];

          # Install phase
          installPhase = ''
            mkdir -p $out/bin
            cp photo-frame $out/bin/
          '';

          # Meta information
          meta = with pkgs.lib; {
            description = "Digital photo frame application";
            platforms = platforms.linux;
          };
        };

        # Native build for development/testing
        photo-frame-native = pkgs.buildGoModule rec {
          pname = "photo-frame";
          version = "0.1.0";

          src = ./.;
          vendorHash = null;

          # Native dependencies for development
          nativeBuildInputs = with pkgs; [
            pkg-config
            xorg.libX11
            xorg.libXcursor
            xorg.libXrandr
            xorg.libXinerama
            xorg.libXi
            xorg.libXxf86vm
            xorg.libGL
            libGL
            libGLU
          ];

          meta = with pkgs.lib; {
            description = "Digital photo frame application (native)";
            platforms = platforms.linux;
          };
        };

      in
      {
        # Development shell
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go development
            go
            gopls
            gotools
            go-tools

            # Cross-compilation tools
            armPkgs.buildPackages.gcc
            armPkgs.buildPackages.pkg-config

            # Native development dependencies for Fyne
            pkg-config
            xorg.libX11
            xorg.libXcursor
            xorg.libXrandr
            xorg.libXinerama
            xorg.libXi
            xorg.libXxf86vm
            libGL
            libGLU

            # Deployment tools
            rsync
            openssh
          ];

          shellHook = ''
            echo "🚀 Photo Frame Development Environment"
            echo ""
            echo "Available commands:"
            echo "  make build-arm    - Cross-compile for ARM"
            echo "  make build-native - Build for current system"
            echo "  make deploy       - Deploy to Pi"
            echo "  make clean        - Clean build artifacts"
            echo ""
            echo "Cross-compilation environment:"
            echo "  GOOS=linux GOARCH=arm GOARM=7"
            echo ""
            
            # Set up cross-compilation environment
            export GOOS=linux
            export GOARCH=arm
            export GOARM=7
            export CGO_ENABLED=1
            export CC=${armPkgs.buildPackages.gcc}/bin/armv7l-unknown-linux-gnueabihf-gcc
            export PKG_CONFIG_PATH=${armPkgs.buildPackages.pkg-config}/bin/pkg-config
          '';
        };

        # Alternative shell for native development only
        devShells.native = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools
            pkg-config
            xorg.libX11
            xorg.libXcursor
            xorg.libXrandr
            xorg.libXinerama
            xorg.libXi
            xorg.libXxf86vm
            libGL
            libGLU
          ];

          shellHook = ''
            echo "🖥️  Native Development Environment"
            echo "Building for: $(go env GOOS)/$(go env GOARCH)"
          '';
        };

        # Package outputs
        packages = {
          default = photo-frame-arm;
          photo-frame-arm = photo-frame-arm;
          photo-frame-native = photo-frame-native;
        };

        # Apps for running
        apps = {
          default = flake-utils.lib.mkApp {
            drv = photo-frame-native;
          };
        };
      }
    );
}
