# A command-line tool: release archives for every platform, checksums,
# a Homebrew formula and a Scoop manifest.
#
#   release/archives/   <name>_<version>_<os>_<arch>.tar.gz (.zip on Windows)
#                       and checksums.txt
#   release/manifests/  <name>.rb (Homebrew), <name>.json (Scoop), rendered
#                       from packaging/ with the archives' checksums
{ pkgs, go, project }:
{ name, version }:
let
  platforms = [
    { goos = "linux"; goarch = "amd64"; }
    { goos = "linux"; goarch = "arm64"; }
    { goos = "darwin"; goarch = "amd64"; }
    { goos = "darwin"; goarch = "arm64"; }
    { goos = "windows"; goarch = "amd64"; }
    { goos = "windows"; goarch = "arm64"; }
  ];

  # The file name of the archive for one platform.
  archiveName = goos: goarch:
    "${name}_${version}_${goos}_${goarch}" + (if goos == "windows" then ".zip" else ".tar.gz");

  # One archive with just the binary, packed reproducibly: fixed timestamps,
  # owners and order, so the same source always gives the same checksum.
  archive = { goos, goarch }:
    let
      bin = go.binary { inherit name version goos goarch; };
      exe = if goos == "windows" then "${name}.exe" else name;
      file = archiveName goos goarch;
    in
    pkgs.runCommand file { nativeBuildInputs = [ pkgs.gnutar pkgs.gzip pkgs.zip ]; } ''
      mkdir pkg
      cp ${bin}/bin/${exe} pkg/
      touch -d 1980-01-01T00:00:00Z pkg/${exe}
      ${if goos == "windows"
        then ''(cd pkg && TZ=UTC zip -X -q $out ${exe})''
        else ''tar --sort=name --mtime=@315532800 --owner=0 --group=0 --numeric-owner -C pkg -cf - ${exe} | gzip -n -9 > $out''}
    '';

  archives = pkgs.linkFarm "${name}-${version}-archives"
    (map (p: let a = archive p; in { inherit (a) name; path = a; }) platforms);

  baseUrl = "https://github.com/${project.owner}/${project.repo}/releases/download/${name}/v${version}";
  homepage = "https://github.com/${project.owner}/${project.repo}";
in
{
  native = go.binary { inherit name version; goos = pkgs.go.GOOS; goarch = pkgs.go.GOARCH; };

  release = pkgs.runCommand "${name}-${version}-release" { } ''
    mkdir -p $out/archives $out/manifests
    cp -L ${archives}/* $out/archives/
    (cd $out/archives && sha256sum * > checksums.txt)

    # checksum <archive>: the archive's SHA-256 from checksums.txt
    checksum() {
      local archive=$1
      awk -v file="$archive" '$2 == file { print $1 }' $out/archives/checksums.txt
    }

    substitute ${../packaging/homebrew}/${name}.rb.in $out/manifests/${name}.rb \
      --subst-var-by version ${version} \
      --subst-var-by baseUrl ${baseUrl} \
      --subst-var-by homepage ${homepage} \
      --subst-var-by sha_darwin_arm64 "$(checksum ${archiveName "darwin" "arm64"})" \
      --subst-var-by sha_darwin_amd64 "$(checksum ${archiveName "darwin" "amd64"})" \
      --subst-var-by sha_linux_arm64 "$(checksum ${archiveName "linux" "arm64"})" \
      --subst-var-by sha_linux_amd64 "$(checksum ${archiveName "linux" "amd64"})"

    substitute ${../packaging/scoop}/${name}.json.in $out/manifests/${name}.json \
      --subst-var-by version ${version} \
      --subst-var-by baseUrl ${baseUrl} \
      --subst-var-by homepage ${homepage} \
      --subst-var-by sha_windows_amd64 "$(checksum ${archiveName "windows" "amd64"})" \
      --subst-var-by sha_windows_arm64 "$(checksum ${archiveName "windows" "arm64"})"

    # Every placeholder must be filled.
    ! grep -E '@[a-zA-Z_]+@|sha256 ""|"hash": ""' $out/manifests/*
  '';
}
