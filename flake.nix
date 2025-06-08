{
  description = "Domino Frame - Cross Compilation";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Cross-compilation target for Pi Zero 2W
        armPkgs = pkgs.pkgsCross.aarch64-multiplatform;

        photo-frame-arm = armPkgs.buildGoModule rec {
          pname = "photo-frame";
          version = "0.1.0";
          src = ./.;

          vendorHash = null;

          ldflags = [ "-s" "-w" ];

          buildInputs = with armPkgs; [
            libxkbcommon
            xorg.libX11.dev

            glfw
            libGL.dev
            libGLU
            openssh
            pkg-config
            glibc
            xorg.libXcursor
            xorg.libXi
            xorg.libXinerama
            xorg.libXrandr
            xorg.libXxf86vm
            xorg.xinput
          ];

          env = {
            CGO_ENABLED = "1";
            GOOS = "linux";
            GOARCH = "arm64";
            CC = "aarch64-unknown-linux-gnu-gcc";

            CGO_CFLAGS = "-I${armPkgs.xorg.libX11.dev}/include -I${armPkgs.libGL.dev}/include";
            CGO_LDFLAGS = "-L${armPkgs.xorg.libX11}/lib -L${armPkgs.xorg.libXcursor}/lib -L${armPkgs.xorg.libXi}/lib -L${armPkgs.xorg.libXinerama}/lib -L${armPkgs.xorg.libXrandr}/lib -L${armPkgs.xorg.libXxf86vm}/lib -L${armPkgs.libGL}/lib";
          };

          meta = with pkgs.lib; {
            description = "Digital photo frame application";
            platforms = platforms.linux;
          };
        };

        photo-frame-native = pkgs.buildGoModule rec {
          pname = "photo-frame";
          version = "0.1.0";

          src = ./.;
          vendorHash = null;

          nativeBuildInputs = with pkgs; [
            pkg-config
            copyDesktopItems
          ];

          buildInputs = with pkgs; [
            libxkbcommon
            xorg.libX11.dev

            glfw
            libGL.dev
            libGLU
            openssh
            pkg-config
            glibc
            xorg.libXcursor
            xorg.libXi
            xorg.libXinerama
            xorg.libXrandr
            xorg.libXxf86vm
            xorg.xinput
          ];

          CGO_ENABLED = "1";

          CGO_CFLAGS = "-I${pkgs.xorg.libX11.dev}/include";
          CGO_LDFLAGS = "-L${pkgs.xorg.libX11}/lib -L${pkgs.libGL}/lib";

          # desktopItems = [
          #   (makeDesktopItem {
          #     name = "photo-frame-native";
          #     exec = pname;
          #     icon = pname;
          #     desktopName = pname;
          #   })
          # ];

          meta = with pkgs.lib; {
            description = "Digital photo frame application (native)";
            platforms = platforms.linux;
          };
        };

      in
      {
        devShells.arm = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools

            armPkgs.buildPackages.gcc
            armPkgs.buildPackages.pkg-config

            pkg-config
            xorg.libX11
            xorg.libXcursor
            xorg.libXrandr
            xorg.libXinerama
            xorg.libXi
            xorg.libXxf86vm
            libGL.dev
            libGLU

            rsync
            openssh
          ];

          env = {
            CGO_ENABLED = "1";
            GOOS = "linux";
            GOARCH = "arm64";
            CC = "aarch64-unknown-linux-gnu-gcc";

            CGO_CFLAGS = "-I${armPkgs.xorg.libX11.dev}/include -I${armPkgs.libGL.dev}/include";
            CGO_LDFLAGS = "-L${armPkgs.xorg.libX11}/lib -L${armPkgs.xorg.libXcursor}/lib -L${armPkgs.xorg.libXi}/lib -L${armPkgs.xorg.libXinerama}/lib -L${armPkgs.xorg.libXrandr}/lib -L${armPkgs.xorg.libXxf86vm}/lib -L${armPkgs.libGL}/lib";
          };

          shellHook = ''
            export GOOS=linux
            export GOARCH=arm64
            export CGO_ENABLED=1

            echo ""
            echo "🖥️ (ARM) Photo Frame Development Environment"
            echo ""
            echo "Available commands:"
            echo "  make build-arm    - Cross-compile for ARM"
            echo "  make build-native - Build for current system"
            echo "  make deploy       - Deploy to Pi"
            echo "  make clean        - Clean build artifacts"
            echo ""
            echo "Cross-compilation environment:"
            echo "  GOOS=$(go env GOOS) GOARCH=$(go env GOARCH)"
            echo ""
          '';
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            go-tools
            glxinfo

            libxkbcommon
            xorg.libX11.dev

            glfw
            libGL.dev
            libGLU
            openssh
            pkg-config
            glibc
            xorg.libXcursor
            xorg.libXi
            xorg.libXinerama
            xorg.libXrandr
            xorg.libXxf86vm
            xorg.xinput
          ];

          CGO_ENABLED = "1";

          CGO_CFLAGS = "-I${pkgs.xorg.libX11.dev}/include";
          CGO_LDFLAGS = "-L${pkgs.xorg.libX11}/lib -L${pkgs.libGL}/lib";

          shellHook = ''
            export GOOS=linux
            export GOARCH=amd64
            export CGO_ENABLED=1
            export PKG_CONFIG_PATH=${pkgs.buildPackages.pkg-config}/bin/pkg-config

            echo ""
            echo "🖥️ (NATIVE) Photo Frame Development Environment"
            echo ""
            echo "Available commands:"
            echo "  make build-arm    - Cross-compile for ARM"
            echo "  make build-native - Build for current system"
            echo "  make deploy       - Deploy to Pi"
            echo "  make clean        - Clean build artifacts"
            echo ""
            echo "Cross-compilation environment:"
            echo "  GOOS=$(go env GOOS) GOARCH=$(go env GOARCH)"
            echo ""
          '';
        };

        packages = {
          default = photo-frame-native;
          photo-frame-arm = photo-frame-arm;
          photo-frame-native = photo-frame-native;
        };

        apps = {
          default = flake-utils.lib.mkApp {
            drv = photo-frame-native;
          };
        };
      }
    );
}
