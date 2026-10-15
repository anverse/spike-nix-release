# A command-line tool: release archives for every platform, checksums,
# a Homebrew formula and a Scoop manifest.
#
#   release/archives/   <name>_<version>_<os>_<arch>.tar.gz (.zip on Windows)
#                       and checksums.txt
#   release/manifests/  <name>.rb (Homebrew), <name>.json (Scoop), rendered
#                       by `release manifests` from packaging/ and the checksums
{ pkgs, go, releaseTool }:
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

  # The project file is the only part of the repository the manifests need.
  projectFile = pkgs.writeTextDir "project.json" (builtins.readFile ../project.json);

  release = pkgs.runCommand "${name}-${version}-release" { } ''
    mkdir -p $out/archives
    cp -L ${archives}/* $out/archives/
    (cd $out/archives && sha256sum * > checksums.txt)

    # Homebrew formula and Scoop manifest from packaging/<tool>/<name>.*.tmpl,
    # with the URLs and checksums of the archives above.
    ${releaseTool}/bin/release manifests \
      -component ${name} -version ${version} -project ${projectFile} \
      -archives $out/archives -templates ${../packaging} -out $out/manifests
  '';
in
{
  native = go.binary { inherit name version; goos = pkgs.go.GOOS; goarch = pkgs.go.GOARCH; };
  inherit release;
}
