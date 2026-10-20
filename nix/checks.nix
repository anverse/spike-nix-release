# Checks of the repository's own configuration, run by `nix flake check`:
# the config files agree, the Nix files are formatted and lint-free, and the
# workflows pass actionlint and zizmor.
{ pkgs, releaseTool }:
let
  inherit (pkgs) lib;
  fileset =
    files:
    lib.fileset.toSource {
      root = ../.;
      fileset = lib.fileset.unions files;
    };
  check =
    name: inputs: src: script:
    pkgs.runCommand "check-${name}" { nativeBuildInputs = inputs; } ''
      cd ${src}
      ${script}
      touch $out
    '';

  nixFiles = lib.fileset.fileFilter (file: file.hasExt "nix") ../.;
  configFiles = fileset [
    ../go.work
    ../project.json
    ../versions.json
    ../nix/go-modules.json
    ../charts
    ../packaging
    (lib.fileset.fileFilter (file: file.name == "go.mod") ../.)
  ];
  workflows = fileset [ ../.github ];
in
{
  # go.work, nix/go-modules.json, project.json and versions.json agree.
  config = check "config" [ pkgs.go ] configFiles ''
    export HOME=$TMPDIR GOTOOLCHAIN=local
    ${releaseTool}/bin/release check
  '';

  nix-format = check "nix-format" [ pkgs.nixfmt ] (fileset [ nixFiles ]) ''
    nixfmt --check $(find . -name '*.nix')
  '';

  nix-lint = check "nix-lint" [ pkgs.statix pkgs.deadnix ] (fileset [ nixFiles ]) ''
    statix check .
    deadnix --fail .
  '';

  workflows = check "workflows" [ pkgs.actionlint pkgs.zizmor ] workflows ''
    actionlint .github/workflows/*.yml
    zizmor --offline --no-progress .github
  '';
}
