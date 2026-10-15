{
  description = "Spike 0011: build and release with Nix, without GoReleaser";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      lib = nixpkgs.lib;
      systems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      forAllSystems = f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      versions = lib.importJSON ./versions.json;
      project = lib.importJSON ./project.json;

      packagesFor = pkgs:
        let
          go = import ./nix/go.nix { inherit pkgs; };
          buildImage = import ./nix/image.nix { inherit pkgs; };
          cli = import ./nix/cli.nix { inherit pkgs go project; };
          service = import ./nix/service.nix { inherit pkgs go buildImage project; };
          gvisor = import ./nix/gvisor.nix { inherit pkgs; };

          spikectl = cli { name = "spikectl"; version = versions.spikectl; };
          spiked = service { name = "spiked"; version = versions.spiked; };
        in
        {
          # spikectl: `nix run .#spikectl`, and its release
          spikectl = spikectl.native;
          spikectl-release = spikectl.release;

          # spiked: the binary for this machine, image, chart, and its release
          spiked = spiked.native;
          spiked-image = spiked.image;
          spiked-chart = spiked.chart;
          spiked-release = spiked.release;

          # Development cluster only: the e2e suite as an image, gVisor for the node
          e2e-image = buildImage {
            name = "e2e";
            version = "dev";
            binaries = {
              amd64 = go.testBinary { version = "dev"; goos = "linux"; goarch = "amd64"; };
              arm64 = go.testBinary { version = "dev"; goos = "linux"; goarch = "arm64"; };
            };
            args = [ "-test.v" ];
          };
          gvisor-aarch64 = gvisor.aarch64;
          distroless = import ./nix/distroless.nix { inherit pkgs; };
          gvisor-x86_64 = gvisor.x86_64;
        };
    in
    {
      packages = forAllSystems packagesFor;

      # `nix flake check` builds every artifact on this machine.
      checks = forAllSystems (pkgs: removeAttrs (packagesFor pkgs) [ "gvisor-aarch64" "gvisor-x86_64" "distroless" ]);

      # Read by the tasks: component kinds, release-note paths, versions.
      lib = {
        inherit versions project;
        pins = lib.importJSON ./pins.json;
        components = import ./nix/components.nix;
      };

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            go-task
            jq
            git-cliff
            gh
            regclient
            kubernetes-helm
            helmfile-wrapped
            kubectl
            k3d
            nix-update
          ];
        };
      });
    };
}
