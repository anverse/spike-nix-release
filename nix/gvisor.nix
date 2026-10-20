# gVisor for the development cluster's node, for the architecture of the
# Docker host: aarch64 or x86_64. Release and hashes come from pins.json;
# `nix run .#update -- gvisor` moves them to the newest release.
{ pkgs }:
let
  pin = (pkgs.lib.importJSON ../pins.json).gvisor;

  gvisor =
    arch:
    pkgs.runCommand "gvisor-${pin.release}-${arch}"
      {
        src = pkgs.fetchurl {
          url = "https://storage.googleapis.com/gvisor/releases/release/${pin.release}/${arch}/gvisor.tar.bz2";
          hash = pin.hash.${arch};
        };
      }
      ''
        mkdir -p $out/bin
        tar -xjf $src -C $out/bin runsc containerd-shim-runsc-v1 gvisor-bin
      '';
in
{
  aarch64 = gvisor "aarch64";
  x86_64 = gvisor "x86_64";
}
