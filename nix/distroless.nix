# The base image of every service image: Google's distroless "static"
# image, user nonroot. It holds CA certificates, time zone data, /etc/passwd
# and nothing else: no shell, no package manager.
#
# Image, digest and hash come from pins.json. Only this download uses the
# network; the hash pins what it downloads. `task update:distroless` moves
# the pin to the newest digest and fixes the hash.
{ pkgs }:
let
  pin = (pkgs.lib.importJSON ../pins.json).distroless;
  image = "${pin.image}:${pin.tag}@${pin.digest}";
in
pkgs.runCommand "distroless-${pin.tag}"
{
  nativeBuildInputs = [ pkgs.regclient pkgs.jq ];
  SSL_CERT_FILE = "${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt";
  outputHashMode = "recursive";
  outputHashAlgo = "sha256";
  outputHash = pin.hash;
} ''
  export HOME=$TMPDIR

  # One image per platform we ship, as an OCI image layout in $out.
  regctl image copy --platform linux/amd64 "${image}" "ocidir://$out:amd64"
  regctl image copy --platform linux/arm64 "${image}" "ocidir://$out:arm64"

  # regctl records the copy time; drop it so the result is reproducible.
  jq 'del(.manifests[].annotations["org.opencontainers.image.created"])' $out/index.json > index.json
  mv index.json $out/index.json
''
