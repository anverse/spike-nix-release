# A container image: the distroless base plus one layer with our binary at
# /usr/local/bin/<name>, for linux/amd64 and linux/arm64, combined into one
# multi-platform image.
#
# The result is an OCI image layout (a directory with index.json and blobs)
# with one tag, <version>. It is built offline by regctl; publishing copies
# it to a registry unchanged.
{ pkgs }:
{
  name,
  version,
  binaries,
  labels ? { },
  args ? [ ],
}:
let
  distroless = import ./distroless.nix { inherit pkgs; };

  entrypoint = builtins.toJSON ([ "/usr/local/bin/${name}" ] ++ args);
  labelFlags = pkgs.lib.concatStringsSep " " (
    pkgs.lib.mapAttrsToList (key: value: "--label ${key}=${value}") labels
  );

  # Shell steps that put our binary on top of the base image for one
  # platform (arch: "amd64" or "arm64").
  addBinary = arch: binary: ''
    # linux/${arch}: a layer with the binary, as a tar with fixed times and owners
    mkdir -p layer-${arch}/usr/local/bin
    cp ${binary}/bin/${name} layer-${arch}/usr/local/bin/${name}
    tar --sort=name --mtime=@315532800 --owner=0 --group=0 --numeric-owner \
      -C layer-${arch} -cf layer-${arch}.tar .
    regctl image mod "ocidir://$out:${arch}" --replace \
      --layer-add "tar=layer-${arch}.tar,platform=linux/${arch}"

    # Entrypoint and labels in a second call: regclient 0.11.6 empties a
    # layer added in the same call as other options. --config-time fixes
    # the time regctl would otherwise record in the image history.
    regctl image mod "ocidir://$out:${arch}" --replace \
      --config-entrypoint '${entrypoint}' ${labelFlags} \
      --config-time set=1970-01-01T00:00:00Z
  '';
in
pkgs.runCommand "${name}-${version}-image"
  {
    nativeBuildInputs = [
      pkgs.regclient
      pkgs.gnutar
      pkgs.jq
    ];
  }
  ''
    export HOME=$TMPDIR

    # Start from the base image (tags amd64 and arm64, see distroless.nix).
    cp -r ${distroless} $out
    chmod -R u+w $out

    ${addBinary "amd64" binaries.amd64}
    ${addBinary "arm64" binaries.arm64}

    # Join both platforms into one image tagged <version>. Then drop the
    # per-platform tags and the creation times regctl records.
    regctl index create "ocidir://$out:${version}" \
      --ref "ocidir://$out:amd64" --ref "ocidir://$out:arm64"
    jq '.manifests |= map(select(.annotations["org.opencontainers.image.ref.name"] == "${version}"))
        | del(.manifests[].annotations["org.opencontainers.image.created"])' \
      $out/index.json > index.json
    mv index.json $out/index.json
  ''
