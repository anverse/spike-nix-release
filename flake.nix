{
  description = "Spike 0011: build and release with Nix";

  # The current stable release branch. `nix run .#update` (weekly in CI)
  # moves flake.lock within the branch; moving to the next release branch is
  # a deliberate change of this line.
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  # The flake's outputs are the interface for people and CI alike:
  #
  #   nix flake check                build every artifact; check every module and the config
  #   nix build .#<artifact>         one artifact, e.g. .#spiked-image
  #   nix run .#release -- <cmd>     prepare, tag and publish releases
  #   nix run .#dev -- <cmd>         development cluster and e2e
  #   nix run .#update               move pinned dependencies to their newest versions
  #   nix develop                    a shell with Go and the tools, for working on the code
  #   nix fmt                        format the Nix files
  #
  # Components come from project.json, versions from versions.json; see nix/.
  outputs =
    { nixpkgs, ... }:
    let
      inherit (nixpkgs) lib;
      systems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      perSystem = lib.genAttrs systems (
        system:
        import ./nix {
          pkgs = nixpkgs.legacyPackages.${system};
          versions = lib.importJSON ./versions.json;
          project = lib.importJSON ./project.json;
        }
      );
    in
    {
      packages = lib.mapAttrs (_: s: s.packages) perSystem;
      apps = lib.mapAttrs (_: s: s.apps) perSystem;
      checks = lib.mapAttrs (_: s: s.checks) perSystem;
      devShells = lib.mapAttrs (_: s: { default = s.devShell; }) perSystem;
      formatter = lib.mapAttrs (_: s: s.formatter) perSystem;
    };
}
