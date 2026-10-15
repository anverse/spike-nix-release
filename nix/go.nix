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
    fileset = lib.fileset.unions [ ../go.mod ../go.sum ../cmd ../internal ../test ];
  };

  # Hash of the Go module dependencies. After changing go.mod, run
  # `task vendor-hash` to update it.
  vendorHash = "sha256-j9mQeUe1+lgmbTGA1s+OKiLbY21qRSfhWTtosc3JVyQ=";

  build = { name, version, goos, goarch, args }:
    (pkgs.buildGoModule ({
      pname = "spike";
      inherit version src vendorHash;
      doCheck = false;
    } // args)).overrideAttrs (old: {
      name = "${name}-${version}-${goos}-${goarch}";
      env = (old.env or { }) // { CGO_ENABLED = "0"; GOOS = goos; GOARCH = goarch; };
    });
in
{
  # The command cmd/<name> for one platform, at $out/bin/<name>[.exe].
  binary = { name, version, goos, goarch }:
    build {
      inherit name version goos goarch;
      args = {
        subPackages = [ "cmd/${name}" ];
        ldflags = [ "-s" "-w" "-X github.com/anverse/spike-nix-release/internal/buildinfo.Version=${version}" ];
        # Cross builds land in bin/<goos>_<goarch>/; move them to bin/.
        postInstall = ''
          if [ -d $out/bin/${goos}_${goarch} ]; then
            mv $out/bin/${goos}_${goarch}/* $out/bin/
            rmdir $out/bin/${goos}_${goarch}
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
}
