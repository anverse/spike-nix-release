# The components the release tasks know: what kind each is, and which
# paths belong to it. A component's release notes list the commits that
# touched these paths since its previous release.
{
  spikectl = {
    kind = "cli";
    paths = [ "cmd/spikectl/**" "internal/buildinfo/**" "packaging/**" "go.mod" "go.sum" ];
  };
  spiked = {
    kind = "service";
    paths = [ "cmd/spiked/**" "internal/buildinfo/**" "charts/spiked/**" "go.mod" "go.sum" ];
  };
}
