# Go builds. Every component is its own Go module (see go.work); a build
# compiles one module, with its own dependencies.
#
# Everything is pure Go (CGO_ENABLED=0), so one host can build for every
# platform by setting GOOS and GOARCH; no SDKs, no Linux builder.
{ pkgs }:
let
  inherit (pkgs) lib;

  # Module directory -> { hash, local }: the hash of its downloaded
  # external dependencies (null without any), and the repository's modules
  # it uses through replace directives. `nix run .#update -- go` writes it;
  # `nix run .#release -- check` checks it against go.work and go.mod.
  goModules = lib.importJSON ./go-modules.json;

  # The source of one module: its directory and its local modules.
  sourceOf =
    module:
    lib.fileset.toSource {
      root = ../.;
      fileset = lib.fileset.unions (
        map (dir: ../. + "/${dir}") ([ module ] ++ goModules.${module}.local)
      );
    };

  # nixpkgs patches its Go toolchain to read time zones (tzdata), service
  # names (iana-etc) and MIME types (mailcap) from /nix/store paths. Our
  # binaries run outside Nix and must use the standard locations
  # (/usr/share/zoneinfo, /etc/services, /etc/mime.types); the store paths
  # also differ per host, so binaries would differ between macOS and Linux.
  # So builds use the same Go without these three patches.
  nixPathPatches = [
    "tzdata"
    "iana-etc"
    "mailcap"
  ];
  goWithStandardPaths = pkgs.go.overrideAttrs (old: {
    patches = builtins.filter (
      patch: !(builtins.any (name: lib.hasInfix name (toString patch)) nixPathPatches)
    ) old.patches;
  });
  buildGoModule = pkgs.buildGoModule.override { go = goWithStandardPaths; };

  # A name for a module in store paths and package names: adapters/echo -> adapters-echo.
  label = module: lib.replaceStrings [ "/" ] [ "-" ] module;

  # The common part of every build of a module. Builds of one module share
  # their dependency download (same pname, same hash) across platforms.
  moduleBuild =
    {
      module,
      version,
      args ? { },
    }:
    buildGoModule (
      {
        pname = "go-" + label module;
        inherit version;
        src = sourceOf module;
        modRoot = module;
        vendorHash = goModules.${module}.hash;
        # Download the module cache rather than a vendor directory: local
        # modules (api, through replace directives) then stay out of the
        # download, so a change to api does not change its hash.
        proxyVendor = true;
        doCheck = false;
        # The Go linker already strips (-s -w) and signs macOS binaries. Nix's
        # fixup (strip, re-signing) differs between Linux and macOS hosts and
        # would make the binaries differ; static Go binaries need none of it.
        dontFixup = true;
        # Builds use the module's own go.mod; go.work is for development. Go
        # never downloads a toolchain of its own.
        env = {
          GOWORK = "off";
          GOTOOLCHAIN = "local";
        };
      }
      // args
    );
in
{
  # A command for one platform, at $out/bin/<name>[.exe]: the main package
  # at `package` (relative to the module) of `module`.
  binary =
    {
      module,
      name,
      version,
      goos,
      goarch,
      package ? ".",
    }:
    let
      # go install names a binary after its package directory.
      built = baseNameOf (if package == "." then module else package);
      exe = if goos == "windows" then ".exe" else "";
    in
    (moduleBuild {
      inherit module version;
      args = {
        subPackages = [ package ];
        ldflags = [
          "-s"
          "-w"
          "-X main.buildVersion=${version}"
        ];
        # What `nix run` starts.
        meta.mainProgram = name;
        postInstall = ''
          # Cross builds land in bin/<goos>_<goarch>/; move them to bin/.
          if [ -d $out/bin/${goos}_${goarch} ]; then
            mv $out/bin/${goos}_${goarch}/* $out/bin/
            rmdir $out/bin/${goos}_${goarch}
          fi
          if [ "${built}" != "${name}" ]; then
            mv $out/bin/${built}${exe} $out/bin/${name}${exe}
          fi
          # A released binary must not depend on anything in the Nix store.
          if grep -q /nix/store $out/bin/*; then
            echo "error: binary contains a /nix/store path" >&2
            exit 1
          fi
        '';
      };
    }).overrideAttrs
      (old: {
        name = "${name}-${version}-${goos}-${goarch}";
        env = old.env // {
          CGO_ENABLED = "0";
          GOOS = goos;
          GOARCH = goarch;
        };
      });

  # The e2e suite (module test/e2e) compiled as a test binary, at $out/bin/e2e.
  testBinary =
    { goos, goarch }:
    (moduleBuild {
      module = "test/e2e";
      version = "dev";
      args = {
        buildPhase = "go test -c -trimpath -o e2e .";
        installPhase = "install -D e2e $out/bin/e2e";
      };
    }).overrideAttrs
      (old: {
        name = "e2e-${goos}-${goarch}";
        env = old.env // {
          CGO_ENABLED = "0";
          GOOS = goos;
          GOARCH = goarch;
        };
      });

  # The dependency download of a module, for `nix run .#update -- go`.
  dependencies =
    module:
    (moduleBuild {
      inherit module;
      version = "deps";
    }).goModules;

  # Modules with dependencies, i.e. with a dependency download.
  modulesWithDependencies = lib.attrNames (lib.filterAttrs (_: m: m.hash != null) goModules);

  # The checks of one module: go.mod and go.sum are tidy, gofmt,
  # golangci-lint (with go vet and gosec, see .golangci.yml) and the tests.
  tests =
    module:
    let
      # A module's own .golangci.yml, else the repository's.
      lintConfig =
        if builtins.pathExists (../. + "/${module}/.golangci.yml") then
          ../. + "/${module}/.golangci.yml"
        else
          ../.golangci.yml;
    in
    moduleBuild {
      inherit module;
      version = "tests";
      args = {
        nativeCheckInputs = [
          pkgs.git
          pkgs.golangci-lint
        ];
        buildPhase = "true";
        doCheck = true;
        checkPhase = ''
          export CGO_ENABLED=0 HOME=$TMPDIR GOLANGCI_LINT_CACHE=$TMPDIR/golangci
          go mod tidy -diff
          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then echo "not gofmt-ed: $unformatted"; exit 1; fi
          golangci-lint run --config ${lintConfig} ./...
          go test ./...
        '';
        installPhase = "touch $out";
      };
    };

  inherit label;
  modules = lib.attrNames goModules;
}
