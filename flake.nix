{
  description = "Spike 0011: build and release with Nix";

  # The current stable release branch. `nix run .#update` (weekly in CI)
  # moves flake.lock within the branch; moving to the next release branch is
  # a deliberate change of this line.
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  # The flake's outputs are the interface for people and CI alike:
  #
  #   nix flake check                build every artifact, run go vet, gofmt, the tests
  #   nix build .#<artifact>         one artifact, e.g. .#spiked-image
  #   nix run .#release -- <cmd>     cut, dry-run (dist) and publish releases
  #   nix run .#dev -- <cmd>         development cluster and e2e
  #   nix run .#update               move pinned dependencies to their newest versions
  #   nix develop                    a shell with Go and the tools, for working on the code
  outputs = { self, nixpkgs }:
    let
      lib = nixpkgs.lib;
      systems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      forAllSystems = f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      versions = lib.importJSON ./versions.json;
      project = lib.importJSON ./project.json;

      # Everything one system can build, as named attributes.
      buildsFor = pkgs:
        let
          goBuilds = import ./nix/go.nix { inherit pkgs; };
          tool = name: goBuilds.binary { inherit name; version = "dev"; goos = pkgs.go.GOOS; goarch = pkgs.go.GOARCH; dir = "tools"; };
          releaseTool = tool "release";

          buildImage = import ./nix/image.nix { inherit pkgs; };
          cli = import ./nix/cli.nix { inherit pkgs releaseTool; go = goBuilds; };
          service = import ./nix/service.nix { inherit pkgs buildImage project; go = goBuilds; };
          gvisor = import ./nix/gvisor.nix { inherit pkgs; };

          spikectl = cli { name = "spikectl"; version = versions.spikectl; };
          spiked = service { name = "spiked"; version = versions.spiked; };

          # A tool as an app: the Go program, with the commands it calls
          # pinned by this flake. Docker and Nix come from the host.
          app = name: description: runtimeInputs: {
            type = "app";
            meta.description = description;
            program = lib.getExe (pkgs.writeShellApplication {
              inherit name runtimeInputs;
              text = ''exec ${tool name}/bin/${name} "$@"'';
            });
          };
        in
        {
          packages = {
            spikectl = spikectl.native;
            spikectl-release = spikectl.release;

            spiked = spiked.native;
            spiked-image = spiked.image;
            spiked-chart = spiked.chart;
            spiked-release = spiked.release;

            # Development cluster only: the e2e suite as an image, gVisor for
            # the node, and the pinned base image on its own for `update`.
            e2e-image = buildImage {
              name = "e2e";
              version = "dev";
              binaries = {
                amd64 = goBuilds.testBinary { version = "dev"; goos = "linux"; goarch = "amd64"; };
                arm64 = goBuilds.testBinary { version = "dev"; goos = "linux"; goarch = "arm64"; };
              };
              args = [ "-test.v" ];
            };
            gvisor-aarch64 = gvisor.aarch64;
            gvisor-x86_64 = gvisor.x86_64;
            distroless = import ./nix/distroless.nix { inherit pkgs; };
          };

          apps = with pkgs; {
            release = app "release" "Cut, dry-run and publish component releases"
              [ git gh regclient kubernetes-helm coreutils ];
            dev = app "dev" "Development cluster with gVisor, and the e2e tests"
              [ k3d kubectl helmfile-wrapped kubernetes-helm regclient coreutils ];
            update = app "update" "Move pinned dependencies to their newest versions"
              [ go git nix-update regclient ];
          };

          checks = { go-tests = goBuilds.tests; };
        };
    in
    {
      packages = forAllSystems (pkgs: (buildsFor pkgs).packages);
      apps = forAllSystems (pkgs: (buildsFor pkgs).apps);

      # Every release artifact plus go vet, gofmt and the unit tests.
      checks = forAllSystems (pkgs:
        let b = buildsFor pkgs; in
        b.checks // removeAttrs b.packages [ "gvisor-aarch64" "gvisor-x86_64" "distroless" ]);

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go gopls go-task git gh regclient kubernetes-helm helmfile-wrapped kubectl k3d nix-update ];
        };
      });
    };
}
