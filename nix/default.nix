# Everything the flake offers for one system: packages, apps, checks and
# the development shell. flake.nix evaluates it once per system.
{
  pkgs,
  versions,
  project,
}:
let
  inherit (pkgs) lib;

  # Not `go`: that name would hide pkgs.go in the `with pkgs;` lists below.
  goBuilds = import ./go.nix { inherit pkgs; };
  buildImage = import ./image.nix { inherit pkgs; };
  gvisor = import ./gvisor.nix { inherit pkgs; };

  # The repository's own tools: module tools, one main package each.
  tool =
    name:
    goBuilds.binary {
      inherit name;
      module = "tools";
      package = name;
      version = "dev";
      goos = pkgs.go.GOOS;
      goarch = pkgs.go.GOARCH;
    };
  releaseTool = tool "release";

  # How each kind of component is built; a library has no artifacts.
  builders = {
    cli = import ./cli.nix {
      inherit pkgs releaseTool;
      go = goBuilds;
    };
    service = import ./service.nix {
      inherit pkgs buildImage project;
      go = goBuilds;
    };
  };

  # The packages of every component in project.json, e.g. spiked,
  # spiked-image, spiked-chart, spiked-release.
  componentPackages = lib.concatMapAttrs (
    name: component:
    lib.optionalAttrs (builders ? ${component.kind}) (
      builders.${component.kind} {
        inherit name;
        inherit (component) module;
        version = versions.${name};
      }
    )
  ) project.components;

  # Development cluster only: the e2e suite as an image, and gVisor for the
  # cluster's node.
  devPackages = {
    e2e-image = buildImage {
      name = "e2e";
      version = "dev";
      binaries = {
        amd64 = goBuilds.testBinary {
          goos = "linux";
          goarch = "amd64";
        };
        arm64 = goBuilds.testBinary {
          goos = "linux";
          goarch = "arm64";
        };
      };
      args = [ "-test.v" ];
    };
    gvisor-aarch64 = gvisor.aarch64;
    gvisor-x86_64 = gvisor.x86_64;
  };

  # Downloads that `nix run .#update` computes hashes for: the pinned base
  # image, and each Go module's dependencies.
  updatePackages = {
    distroless = import ./distroless.nix { inherit pkgs; };
  }
  // lib.listToAttrs (
    map (
      module: lib.nameValuePair "go-deps-${goBuilds.label module}" (goBuilds.dependencies module)
    ) goBuilds.modulesWithDependencies
  );

  # A tool as an app: the Go program, with the commands it calls pinned by
  # the flake. Docker and Nix come from the host.
  app = name: description: runtimeInputs: {
    type = "app";
    meta.description = description;
    program = lib.getExe (
      pkgs.writeShellApplication {
        inherit name runtimeInputs;
        text = ''
          export GOTOOLCHAIN=local
          exec ${tool name}/bin/${name} "$@"
        '';
      }
    );
  };
in
{
  packages = componentPackages // devPackages // updatePackages;

  apps = with pkgs; {
    release = app "release" "Prepare, tag and publish component releases" [
      go
      git
      gh
      regclient
      kubernetes-helm
      coreutils
    ];
    dev = app "dev" "Development cluster with gVisor, and the e2e tests" [
      k3d
      kubectl
      helmfile-wrapped
      kubernetes-helm
      regclient
      coreutils
    ];
    update = app "update" "Move pinned dependencies to their newest versions" [
      go
      git
      regclient
      cosign
    ];
  };

  # Every component's artifacts, each Go module's checks, and the checks of
  # the repository's own configuration.
  checks =
    componentPackages
    // {
      inherit (devPackages) e2e-image;
    }
    // lib.listToAttrs (
      map (
        module: lib.nameValuePair "go-${goBuilds.label module}" (goBuilds.tests module)
      ) goBuilds.modules
    )
    // import ./checks.nix { inherit pkgs releaseTool; };

  devShell = pkgs.mkShell {
    packages = with pkgs; [
      go
      gopls
      golangci-lint
      go-task
      git
      gh
      regclient
      kubernetes-helm
      helmfile-wrapped
      kubectl
      k3d
      nixfmt
    ];
    # Go never downloads a toolchain of its own.
    GOTOOLCHAIN = "local";
  };

  formatter = pkgs.nixfmt-tree or pkgs.nixfmt;
}
