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
        # armPkgs = pkgs.pkgsCross.armv7l-hf-multiplatform;
        armPkgs = pkgs.pkgsCross.aarch64-multiplatform;

        # Build the photo frame binary for ARM
        photo-frame-arm = armPkgs.buildGoModule rec {
          pname = "photo-frame";
          version = "0.1.0";

          src = ./.;

          # You'll need to update this hash after first build
          vendorHash = null;
          buildFlags = [ "-mod=readonly" ];

          # Build inputs for ARM target
          # buildInputs = with armPkgs; [
          #   libxkbcommon
          #   xorg.libX11.dev
          #
          #   glfw
          #   libGL.dev
          #   libGLU
          #   openssh
          #   pkg-config
          #   glibc
          #   xorg.libXcursor
          #   xorg.libXi
          #   xorg.libXinerama
          #   xorg.libXrandr
          #   xorg.libXxf86vm
          #   xorg.xinput
          # ];
          #
          nativeBuildInputs = with armPkgs.buildPackages; [
            pkg-config
            gcc
          ];

          # stdenv = armPkgs.llvmPackages_18.stdenv;

          # CGO settings for cross-compilation
          # env = {
          #   CGO_ENABLED = "1";
          #   GOOS = "linux";
          #   GOARCH = "arm64";
          #
          #   # Point to the correct ARM libraries
          #   CGO_CFLAGS = "-I${armPkgs.xorg.libX11.dev}/include -I${armPkgs.libGL.dev}/include";
          #   CGO_LDFLAGS = "-L${armPkgs.xorg.libX11}/lib -L${armPkgs.xorg.libXcursor}/lib -L${armPkgs.xorg.libXi}/lib -L${armPkgs.xorg.libXinerama}/lib -L${armPkgs.xorg.libXrandr}/lib -L${armPkgs.xorg.libXxf86vm}/lib -L${armPkgs.libGL}/lib";
          #
          #   # Cross-compilation toolchain
          #   CC = "${armPkgs.buildPackages.gcc}/bin/aarch64-unknown-linux-gnu-gcc";
          #   CXX = "${armPkgs.buildPackages.gcc}/bin/aarch64-unknown-linux-gnu-g++";
          #
          #   # PKG_CONFIG for cross-compilation
          #   PKG_CONFIG = "${armPkgs.buildPackages.pkg-config}/bin/pkg-config";
          #   PKG_CONFIG_PATH = "${armPkgs.xorg.libX11.dev}/lib/pkgconfig:${armPkgs.libGL.dev}/lib/pkgconfig";
          # };
          # GOOS = "linux";
          # GOARCH = "arm64";
          # GOARM = "7";

          # Point to ARM libraries

          # Build flags for smaller binary
          # ldflags = [ "-s" "-w" ];

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

          # Let Go handle cross-compilation internally
          env = {
            CGO_ENABLED = "1";
            GOOS = "linux";
            GOARCH = "arm64";

            CGO_CFLAGS = "-I${armPkgs.xorg.libX11.dev}/include";
            CGO_LDFLAGS = "-L${armPkgs.xorg.libX11}/lib -L${armPkgs.libGL}/lib";
            # Only set CGO flags for libraries, not CC
            # CGO_CFLAGS = "-I${armPkgs.xorg.libX11.dev}/include -I${armPkgs.libGL.dev}/include";
            # CGO_LDFLAGS = "-L${armPkgs.xorg.libX11}/lib -L${armPkgs.xorg.libXcursor}/lib -L${armPkgs.xorg.libXi}/lib -L${armPkgs.xorg.libXinerama}/lib -L${armPkgs.xorg.libXrandr}/lib -L${armPkgs.xorg.libXxf86vm}/lib -L${armPkgs.libGL}/lib";
          };

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

          # GOOS = "linux";
          # GOARCH = "amd64";

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

          # Build flags for smaller binary
          ldflags = [ "-s" "-w" ];

          CGO_ENABLED = "1";

          # Point to X11/libGL libraries
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
        # Development shell
        devShells.arm = pkgs.mkShell {
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
            libGL.dev
            libGLU

            # Deployment tools
            rsync
            openssh
          ];

          shellHook = ''
            # Set up cross-compilation environment
            export GOOS=linux
            export GOARCH=arm64
            # export GOARM=7
            export CGO_ENABLED=1
            # export CC=${armPkgs.buildPackages.gcc}/bin/aarch64-unknown-linux-gnu-gcc
            # export PKG_CONFIG_PATH=${armPkgs.buildPackages.pkg-config}/bin/pkg-config

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
            echo "  GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) GOARM=$(go env GOARM)"
            echo ""
          '';
        };

        # Alternative shell for native development only
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # go
            # gopls
            # gotools
            # go-tools
            # pkg-config
            # xorg.libX11
            # xorg.libXcursor
            # xorg.libXrandr
            # xorg.libXinerama
            # xorg.libXi
            # xorg.libXxf86vm
            # libGL
            # libGLU

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
            # wayland.dev
            # mesa
          ];

          # CGO settings for cross-compilation
          CGO_ENABLED = "1";

          # Point to X11/libGL libraries
          CGO_CFLAGS = "-I${pkgs.xorg.libX11.dev}/include";
          CGO_LDFLAGS = "-L${pkgs.xorg.libX11}/lib -L${pkgs.libGL}/lib";

          shellHook = ''
            # Set up cross-compilation environment
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
            echo "  GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) GOARM=$(go env GOARM)"
            echo ""
          '';
        };

        # Package outputs
        packages = {
          default = photo-frame-native;
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
