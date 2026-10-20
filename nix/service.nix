# A service: Linux binaries, a container image and a Helm chart, as the
# packages <name>, <name>-image, <name>-chart and <name>-release.
#
#   image    multi-platform image (linux/amd64, linux/arm64), see image.nix
#   chart    charts/service, named after the component, with version and
#            appVersion = <version> and the image's digest as default
#   release  image/ and chart/ together, what the release pipeline publishes
{
  pkgs,
  go,
  buildImage,
  project,
}:
{
  name,
  version,
  module,
}:
let
  binary =
    goarch:
    go.binary {
      inherit
        module
        name
        version
        goarch
        ;
      goos = "linux";
    };
  repository = "${project.registry}/${name}";

  image = buildImage {
    inherit name version;
    binaries = {
      amd64 = binary "amd64";
      arm64 = binary "arm64";
    };
    labels = {
      "org.opencontainers.image.source" = "https://github.com/${project.owner}/${project.repo}";
      "org.opencontainers.image.version" = version;
    };
  };

  # The shared chart, filled in for this component. helm package validates
  # it and sets its version. Helm 3 stamps the files with the build time, so
  # the archive is packed again with fixed times, owners and order.
  chart =
    pkgs.runCommand "${name}-${version}-chart"
      {
        nativeBuildInputs = [
          pkgs.kubernetes-helm
          pkgs.gnutar
          pkgs.gzip
          pkgs.jq
        ];
      }
      ''
        export HOME=$TMPDIR
        archive=${name}-${version}.tgz
        digest=$(jq -r '.manifests[0].digest' ${image}/index.json)

        cp -r ${../charts/service} ${name}
        chmod -R u+w ${name}
        substituteInPlace ${name}/Chart.yaml \
          --replace-fail '"@name@"' '${name}' \
          --replace-fail '"@description@"' '"${name}, a service of ${project.repo}"'
        substituteInPlace ${name}/values.yaml \
          --replace-fail '"@repository@"' '${repository}' \
          --replace-fail '"@digest@"' "\"$digest\""

        helm lint ${name}
        helm package ${name} --version ${version} --app-version ${version}

        mkdir unpacked
        tar -xzf $archive -C unpacked
        mkdir $out
        tar --sort=name --mtime=@315532800 --owner=0 --group=0 --numeric-owner \
          -C unpacked -cf - ${name} | gzip -n -9 > $out/$archive
        helm lint $out/$archive
      '';
in
{
  ${name} = go.binary {
    inherit module name version;
    goos = pkgs.go.GOOS;
    goarch = pkgs.go.GOARCH;
  };
  "${name}-image" = image;
  "${name}-chart" = chart;
  "${name}-release" = pkgs.linkFarm "${name}-${version}-release" [
    {
      name = "image";
      path = image;
    }
    {
      name = "chart";
      path = chart;
    }
  ];
}
