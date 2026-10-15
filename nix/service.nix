# A service: Linux binaries, a container image and a Helm chart.
#
#   image    multi-platform image (linux/amd64, linux/arm64), see image.nix
#   chart    charts/<name>, packaged with version and appVersion = <version>
#   release  image/ and chart/ together, what the release pipeline publishes
{ pkgs, go, buildImage, project }:
{ name, version }:
let
  binary = goarch: go.binary { inherit name version goarch; goos = "linux"; };

  image = buildImage {
    inherit name version;
    binaries = { amd64 = binary "amd64"; arm64 = binary "arm64"; };
    labels = {
      "org.opencontainers.image.source" = "https://github.com/${project.owner}/${project.repo}";
      "org.opencontainers.image.version" = version;
    };
  };

  chart = pkgs.runCommand "${name}-${version}-chart"
    { nativeBuildInputs = [ pkgs.kubernetes-helm ]; }
    ''
      export HOME=$TMPDIR
      helm lint ${../charts}/${name}
      helm package ${../charts}/${name} --version ${version} --app-version ${version} -d $out
    '';
in
{
  native = go.binary { inherit name version; goos = pkgs.go.GOOS; goarch = pkgs.go.GOARCH; };
  inherit image chart;
  release = pkgs.linkFarm "${name}-${version}-release" [
    { name = "image"; path = image; }
    { name = "chart"; path = chart; }
  ];
}
