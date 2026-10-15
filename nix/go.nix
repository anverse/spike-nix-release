# Go builds.
#
# Everything is pure Go (CGO_ENABLED=0), so one host can build for every
# platform by setting GOOS and GOARCH; no SDKs, no Linux builder.
# All builds share one Go module download (same pname, same vendorHash).
{ pkgs }:
let
  inherit (pkgs) lib;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [ ../go.mod ../go.sum ../cmd ../internal ../test ../tools ];
  };

  # nixpkgs patches its Go toolchain to read time zones (tzdata), service
  # names (iana-etc) and MIME types (mailcap) from /nix/store paths. Our
  # binaries run outside Nix and must use the standard locations
  # (/usr/share/zoneinfo, /etc/services, /etc/mime.types); the store paths
  # also differ per host, so binaries would differ between macOS and Linux.
  # So release builds use the same Go without these three patches.
  nixPathPatches = [ "tzdata" "iana-etc" "mailcap" ];
  goWithStandardPaths = pkgs.go.overrideAttrs (old: {
    patches = builtins.filter
      (patch: !(builtins.any (name: lib.hasInfix name (toString patch)) nixPathPatches))
      old.patches;
  });
  buildGoModule = pkgs.buildGoModule.override { go = goWithStandardPaths; };

  # Hash of the Go module dependencies. `nix run .#update -- go` updates the
  # modules and this hash.
  vendorHash = "sha256-j9mQeUe1+lgmbTGA1s+OKiLbY21qRSfhWTtosc3JVyQ=";

  build = { name, version, goos, goarch, args }:
    (buildGoModule ({
      pname = "spike";
      inherit version src vendorHash;
      doCheck = false;
      # The Go linker already strips (-s -w) and signs macOS binaries. Nix's
      # fixup (strip, re-signing) differs between Linux and macOS hosts and
      # would make the binaries differ; static Go binaries need none of it.
      dontFixup = true;
    } // args)).overrideAttrs (old: {
      name = "${name}-${version}-${goos}-${goarch}";
      env = (old.env or { }) // { CGO_ENABLED = "0"; GOOS = goos; GOARCH = goarch; };
    });
in
{
  # A command for one platform, at $out/bin/<name>[.exe]. Commands live in
  # cmd/<name>, the repository's own tools in tools/<name>.
  binary = { name, version, goos, goarch, dir ? "cmd" }:
    build {
      inherit name version goos goarch;
      args = {
        subPackages = [ "${dir}/${name}" ];
        # What `nix run` starts; pname is shared by all builds.
        meta.mainProgram = name;
        ldflags = [ "-s" "-w" "-X github.com/anverse/spike-nix-release/internal/buildinfo.Version=${version}" ];
        # Cross builds land in bin/<goos>_<goarch>/; move them to bin/.
        postInstall = ''
          if [ -d $out/bin/${goos}_${goarch} ]; then
            mv $out/bin/${goos}_${goarch}/* $out/bin/
            rmdir $out/bin/${goos}_${goarch}
          fi
          # A released binary must not depend on anything in the Nix store.
          if grep -q /nix/store $out/bin/*; then
            echo "error: binary contains a /nix/store path" >&2
            exit 1
          fi
        '';
      };
    };

  # The end-to-end suite compiled as a test binary, at $out/bin/e2e.
  testBinary = { version, goos, goarch }:
    build {
      name = "e2e";
      inherit version goos goarch;
      args = {
        buildPhase = "go test -c -trimpath -o e2e ./test/e2e";
        installPhase = "install -D e2e $out/bin/e2e";
      };
    };

  # go vet, gofmt and the unit tests, as a check: `nix flake check` runs it.
  tests = (pkgs.buildGoModule {
    pname = "spike";
    version = "tests";
    inherit src vendorHash;
    nativeCheckInputs = [ pkgs.git ];
    buildPhase = "true";
    checkPhase = ''
      export CGO_ENABLED=0 HOME=$TMPDIR
      go vet ./...
      unformatted=$(gofmt -l cmd internal test tools)
      if [ -n "$unformatted" ]; then echo "not gofmt-ed: $unformatted"; exit 1; fi
      go test ./...
    '';
    installPhase = "touch $out";
  });
}
