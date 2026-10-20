# Spike 0011: build and release with Nix, without GoReleaser

| | |
|---|---|
| Status | Done. Part A 2026-10-14 (darwin/arm64, local), part B 2026-10-15 (GitHub, [results](results/part-b-github.md)), part C 2026-10-15 (flake apps, Go tools, stable nixpkgs), part D 2026-10-15 (one Go module per component, cobra). Package visibility and cleanup open. |
| Question | Can a Nix flake build every artifact of a monorepo (a CLI for three operating systems, multi-platform images, a Helm chart, Homebrew and Scoop manifests), with Taskfile as the only entry point and small CLIs publishing one GitHub release per component tag, identically on a laptop and in CI? |
| Feeds into (agentd spec) | ADR-0023 (Nix as the single build system), ADR-0024 (releases per component), ADR-0019 and development.md (environment, e2e as Helm tests, CI), milestone v0.0, roadmap open question 28 |
| Builds on | Spike 0002 (k3d with gVisor) |
| Versions | nixpkgs nixos-26.05 (part C; parts A and B: unstable): Go 1.26.8, regclient 0.11.5, Helm 3.20.2, helmfile 1.5.0, k3d 5.8.3, nix-update 1.15.1, gh 2.102.0; distroless `static-debian13:nonroot` (pinned by digest); k3s v1.34.12-k3s1; gVisor 20261005; Nix 2.34 |

The spike is a stand-in project with two components and no agentd name, so no agentd package or release exists before the real one: `spikectl` (a CLI, like `agentctl`) and `spiked` (a service with image and chart, like `agentd`). The directory is laid out as a repository root; part B runs it as its own GitHub repository, `anverse/spike-nix-release`, publishing into the existing `anverse/homebrew-tap` and `anverse/scoop-bucket`.

## Hypotheses

1. **Cross builds.** Nix's `buildGoModule` with `CGO_ENABLED=0` and `GOOS`/`GOARCH` overrides builds the CLI for linux, darwin and windows on amd64 and arm64, and the service for linux on both, from macOS and from Linux, without SDKs or a Linux builder.
2. **Images without Docker or a Linux builder.** A tool from nixpkgs assembles a multi-platform OCI image (linux/amd64 + linux/arm64 in one index) offline, inside a Nix build: the distroless `static` base plus a layer with our binary. Both platforms run.
3. **Reproducible.** Archives, manifests, chart and images are bit-for-bit identical when rebuilt, and across machines.
4. **Manifests from archives.** The Homebrew formula and the Scoop manifest are rendered in Nix from the archives they describe, so hashes always match.
5. **Dependency hash.** `nix-update` updates the module's shared `vendorHash` after a dependency change.
6. **Release cutting and notes.** One task bumps `versions.json`, commits, tags `<component>/vX.Y.Z` and pushes. Notes list only the commits that touched the component's paths, without a commit convention.
7. **e2e as Helm tests.** A declared k3d cluster with gVisor and a local registry, deployed with helmfile, runs the e2e suite as a Helm test pod under gVisor; same command locally and in CI.
8. **Updates.** Every external input is pinned (flake.lock, go.sum with vendorHash, pins.json for distroless, gVisor, k3s); one task finds newer versions upstream and fixes the hashes, and a weekly workflow turns that into a pull request.
9. **GitHub (part B).** Slash tags work as release tags and in download URLs; only the CLI's releases become "latest"; the pipeline publishes images and charts to ghcr.io and the formula and manifest to the existing tap and bucket with the organization's release bot apps; users install with Homebrew (macOS, Linux), Scoop (Windows amd64 and arm64) and Nix; gVisor in k3d runs on GitHub's amd64 and arm64 runners; CI times with a cached Nix store are acceptable.

## Setup

The flake's outputs are the interface: packages, checks and apps. CI calls nothing else. Every component is its own Go module; components and their kinds come from `project.json`.

```
go.work                   all Go modules, for development; builds use each module's go.mod
api/                      module, library: the shared contract (stands in for the Session API)
spikectl/                 module, CLI (stands in for agentctl)
spiked/                   module, service (stands in for agentd)
adapters/echo/            module, service spiked-echo: an adapter with a dependency of its own (uuid)
tools/                    module: release, dev, update (cobra), with tests and its own .golangci.yml
test/e2e/                 module: the e2e suite, run as a Helm test against both services
flake.nix                 evaluates nix/ once per system
nix/default.nix           packages, apps, checks and the shell for one system
nix/checks.nix            config consistency, nixfmt, statix, deadnix, actionlint, zizmor
nix/go.nix                Go builds per module, and per module: tidy, gofmt, golangci-lint, tests
nix/cli.nix, service.nix  how a CLI and a service are built and released
nix/image.nix             an image: distroless base + our binary, both platforms, built by regctl
nix/distroless.nix, gvisor.nix   pinned downloads
nix/go-modules.json       per module: hash of its external dependencies, its local modules
project.json              components (kind, module, extra paths), repository, registry, tap, bucket
versions.json             version per component (written by `release prepare`)
pins.json                 distroless (digest, hash), gVisor (release, hashes), k3s (tag, digest)
charts/service/           the one chart of every service; Nix sets name, version and image digest
packaging/                Homebrew formula and Scoop manifest as Go templates
dev/                      k3d config, containerd template (gVisor), helmfile (local, ci)
.golangci.yml             linters for the Go modules (standard + gosec)
.github/                  workflows, the nix action, dependabot, zizmor.yml, CODEOWNERS
Taskfile.yml              optional shortcuts for people
```

## How to run

```sh
nix flake check                                  # every artifact; every module and the config checked
nix run .#release -- dist spiked                 # a release into dist/, without publishing
nix run .#dev -- e2e                             # k3d + gVisor + registry, both services, Helm tests
nix run .#dev -- down
nix run .#update                                 # move every pin to its newest version
nix run .#release -- prepare spiked 0.4.0        # release PR; after merge:
nix run .#release -- tag spiked "Summary."       # tag the merged commit; the tag starts the workflow
```

## Repository settings

These settings protect releases but live outside the repository; manage them where the repository is created (Terraform, `integrations/github` provider):

```hcl
# main takes changes only through pull requests with green checks.
resource "github_repository_ruleset" "main" {
  repository  = "spike-nix-release"
  name        = "main"
  target      = "branch"
  enforcement = "active"
  conditions {
    ref_name {
      include = ["~DEFAULT_BRANCH"]
      exclude = []
    }
  }
  rules {
    deletion         = true
    non_fast_forward = true
    pull_request {
      # 0 for a single maintainer, who cannot approve their own pull request.
      required_approving_review_count = 0
      require_code_owner_review       = true
    }
    required_status_checks {
      strict_required_status_checks_policy = true
      required_check { context = "compare" }
      required_check { context = "e2e (ubuntu-24.04)" }
      required_check { context = "e2e (ubuntu-24.04-arm)" }
    }
  }
}

# Only maintainers create, move or delete release tags.
resource "github_repository_ruleset" "release_tags" {
  repository  = "spike-nix-release"
  name        = "release tags"
  target      = "tag"
  enforcement = "active"
  conditions {
    ref_name {
      include = ["refs/tags/*/v*"]
      exclude = []
    }
  }
  bypass_actors {
    actor_id    = 5 # repository admins
    actor_type  = "RepositoryRole"
    bypass_mode = "always"
  }
  rules {
    creation = true
    update   = true
    deletion = true
  }
}

# The release environment holds the bot secrets; only release tags deploy to it.
resource "github_repository_environment" "release" {
  repository  = "spike-nix-release"
  environment = "release"
  deployment_branch_policy {
    protected_branches     = false
    custom_branch_policies = true
  }
}

resource "github_repository_environment_deployment_policy" "release_tags" {
  repository  = "spike-nix-release"
  environment = github_repository_environment.release.environment
  tag_pattern = "*/v*"
}

# HOMEBREW_BOT_APP_ID, HOMEBREW_BOT_PRIVATE_KEY, SCOOP_BOT_APP_ID and
# SCOOP_BOT_PRIVATE_KEY as secrets of the release environment, instead of
# organization secrets every workflow of the repository can read.
resource "github_actions_environment_secret" "homebrew_bot_app_id" {
  repository      = "spike-nix-release"
  environment     = github_repository_environment.release.environment
  secret_name     = "HOMEBREW_BOT_APP_ID"
  plaintext_value = var.homebrew_bot_app_id
}
# … the same for the other three.
```

Further settings:

- **Update bot**: a GitHub App with Contents and Pull requests (read and write) on this repository; its ID and key as `UPDATE_BOT_APP_ID` and `UPDATE_BOT_PRIVATE_KEY`. The `update` workflow opens its pull request with it, so the checks run. The app is created by hand; its repositories and secrets through `github_app_installation_repositories` and `github_actions_secret`.
- **Package visibility**: each image and chart package is made public once, in its package settings (no API).
- **Code owners**: `.github/CODEOWNERS` names who reviews workflows, build definitions, pins and the release tool.

## Results (part A)

Parts A and B ran with an earlier layout: Taskfile tasks held the release, update and cluster logic, and git-cliff wrote the notes. Part C replaced them; see below.

All on darwin/arm64 (Rancher Desktop, moby), no Linux builder.

| # | Result |
|---|---|
| 1 | All six CLI targets and both service targets build from macOS. Linux binaries are static ELF; the darwin/arm64 one runs. A full build of every artifact takes about 20 s with a warm Nix store. |
| 2 | **regctl** (regclient) builds the image offline inside a Nix derivation. The distroless base is fetched once by digest (a fixed-output derivation, the only network access) for linux/amd64 and linux/arm64; per platform, one layer with the binary is added and entrypoint and OCI labels set; `index create` joins both. The result is an OCI image layout with one tag. A first version without a base image (binary, CA bundle, `/etc/passwd` from Nix) ran under Docker on both platforms as user 65532; the distroless version runs in the e2e cluster. |
| 3 | `nix build --rebuild` reproduces archives, manifests, image layout, chart and e2e image bit for bit, and the distroless fetch matches its pinned hash. Across machines: part B. |
| 4 | Formula and manifest carry the hashes from `checksums.txt`; a missing hash fails the build. |
| 5 | `nix-update --flake --version=skip spikectl` replaces the placeholder `vendorHash` in `nix/go.nix`; all components share it. |
| 6 | `task spike:verify-notes` ([log](results/notes-fixture.log)) cuts `spikectl` 0.1.0, `spiked` 0.1.0 and `spikectl` 0.2.0 in a throwaway repository. Each release commit holds only `versions.json`; only the released component is built. Notes per tag list exactly the commits touching the component's paths: a `spiked` change never appears in `spikectl`'s notes, a shared package change appears in both, release commits are skipped. |
| 7 | `task e2e` from nothing to a passing Helm test in 28 to 36 s ([log](results/e2e-local-darwin-arm64.log)): k3d with gVisor and a registry from one config file, helmfile deploys `spiked` and the e2e release under gVisor, `helmfile test --logs` runs the Go test binary in a pod, which reaches the service and confirms it runs under gVisor. |
| 8 | `task update:distroless` finds the newest digest with `regctl manifest digest`, builds with an empty hash and takes the hash Nix reports; from a stale digest and wrong hash it restores both. `update:gvisor` reads the latest GitHub release and prefetches both tarballs (`nix store prefetch-file`). `update:k3s` reads the k3s release channel (found v1.36.5 on `stable`, pinned v1.34.12). Each task does nothing when its pin is current. The weekly workflow: part B. |
| 9 | Pending. |

## Findings

1. **No Linux builder is needed on macOS** when images contain only static Go binaries and data. Nix cross-compiles the binaries; regctl writes the image layout as files. nixpkgs `dockerTools` was not needed.
2. **regclient 0.11.6 empties a newly added layer** when `--layer-add` is combined with layer options (`--layer-compress`, `--reproducible`, `--time`) in one `image mod` call. Adding the layer in one call and setting the config in a second works. The default gzip output is already deterministic for a deterministic tar. Each added layer gets a history entry with the current time; `--config-time` pins it (and the image's creation time) without touching layers. The copy into an image layout also records its time in `index.json`, which the build removes.
3. **Helm tests run the real system.** A Helm test is a pod annotated `helm.sh/hook: test`; `helm test` starts it against the deployed release and reports its exit code. Here it is a dedicated e2e chart, so the product chart carries no test code.
4. **k3d** expands environment variables in its config file (used for the gVisor mounts, which need absolute paths). It names a registry created from the config exactly as given (`spike-registry`, no `k3d-` prefix); pods pull from `spike-registry:5000`, the host pushes to `127.0.0.1:5111`. Docker on macOS cannot mount `/nix/store`, so gVisor is copied from the Nix store into `.cache/` first.
5. **git-cliff needs an explicit range** per component (`<previous tag>..<tag>`, or the tag's commit for a first release). `--current` fails without a previous tag, and `--latest` takes the newest tag in the repository, not the checked-out one. The release task computes the previous tag with `git describe --match '<component>/v*'`.
6. **Embedding the commit in binaries** makes every commit rebuild every artifact, and a component's release artifacts depend on unrelated commits. The spike now embeds the version only.
7. **Go modules are downloaded once** when every build shares the same `pname` and `vendorHash`; with a `pname` per target, each target had its own module download.
8. **Commit signing**: the release task creates commits and tags with the maintainer's git configuration, signing included; the throwaway repository in `spike:verify-notes` disables signing.
9. Helm 4.3 works with helmfile 1.8 (`sync`, `test`); `apply` would need the helm-diff plugin.
10. **Taskfile includes** keep the root readable: included tasks run in the root directory and call each other without their namespace; `task release` runs the included file's `default` task.
11. **distroless vs. a Nix-built base**: both are tiny. distroless brings time zones and package metadata for scanners and is maintained upstream; it costs one pinned download, updated by changing digest and hash.
12. **Fixed-output downloads need their hash updated with the version.** For a single file `nix store prefetch-file` returns it; for the image layout, a build with an empty hash reports it. Both are one task step.
13. **Pull requests opened with the workflow token start no workflows**; the update workflow dispatches CI and e2e on its branch instead (dispatch is the documented exception).

## Not covered

Signing and provenance of images and archives; macOS notarization; Sandbox reference images for agent programs (v0.1); the platform stack (Keycloak, CloudNativePG, OpenBao, agentgateway) in the dev environment, which v0.0 adds on top of this setup.

## Part C: flake outputs as the only interface

Rules applied: everything a person or CI does is a flake package, check or app; release, update and cluster logic are small Go programs (`tools/`) with unit tests instead of Taskfile shell; nixpkgs follows the stable `nixos-26.05` branch; the GitHub Actions are on their current major versions.

| Result |
|---|
| CI calls only `nix flake check`, `nix run .#release -- dist`, `nix run .#dev -- e2e`, `nix run .#release -- login/publish` and `nix run .#update`. `nix flake check` builds every artifact and runs go vet, gofmt and the unit tests (notes against a temporary git repository, manifest rendering, tag parsing). |
| Releases `spikectl` 0.2.0 and `spiked` 0.2.0 cut with `nix run .#release -- cut`: all install jobs green (Homebrew on 4 runners, Scoop on 2, Nix on 2, `releases/latest`), the image runs from ghcr.io on amd64 and arm64 and the chart renders from the registry. Release job about 1 min. |
| Tap and bucket commits read `Brew formula update for spikectl version v0.2.0` and `Scoop update for spikectl version v0.2.0`. |
| With a warm Nix cache: `ci` 0.5 to 2 min per runner, `e2e` about 1.2 min per architecture; artifacts identical across the three runners; no warnings in the logs after the action updates. |

### Findings (part C)

21. **The Go tools replace about 250 lines of Taskfile shell** with code that has types, error messages and unit tests. git-cliff is no longer needed: `git log <previous tag>..<tag> -- <paths>` in about 40 lines does what the notes need.
22. **Helm 3 (stable branch) stamps chart files with the build time**, so `helm package` is not reproducible; Helm 4 on unstable was. The chart is packed again with fixed times, owners and order after `helm package` has validated it and set the version.
23. **`with pkgs;` does not shadow `let` bindings**: a local `go` hid `pkgs.go` in an app's runtime inputs. Local names now avoid package names (`goBuilds`).
24. **Cluster DNS is not ready when the node is**: one local e2e run failed with refused DNS lookups. `dev up` now waits for the CoreDNS deployment.
25. **Release-note paths cover source, not build files.** Changes to `nix/`, `flake.lock` or `pins.json` change artifacts but appear in no component's notes. Acceptable for now; a component could list them if its users care.
26. **Logging in from a job** (`docker login`, `helm registry login` with the workflow token) lets the pull test work while packages are private.

## Part D: one Go module per component

Every component is its own Go module: `spikectl`, `spiked`, the adapter `spiked-echo` (`adapters/echo`, with `github.com/google/uuid` that no other module carries), plus `api` (the shared contract), `tools` and `test/e2e`. Components use only `api`, through a replace directive; `go.work` ties the modules together for development. The tools use cobra and are written as numbered steps with named helpers.

| Result |
|---|
| `nix flake check` builds and tests each module on its own: six test checks (vet, gofmt, tests) plus every artifact. Builds use each module's `go.mod` (`GOWORK=off`). |
| `nix run .#update -- go` updates each module separately: it moved only `tools` (pflag 1.0.9 to 1.0.10), and only the `tools` hash changed. |
| Notes follow modules: an `api` change appears in `spiked` and `spiked-echo`, an adapter change only in `spiked-echo`, neither in `spikectl`. Releases `spiked-echo` 0.1.0, `spiked` 0.3.0 and `spikectl` 0.3.0 passed every pull and install check. |
| The release tool's help and completion come from cobra (`nix run .#release -- --help`). |

### Findings (part D)

27. **Per-module builds work with nixpkgs' `buildGoModule`**: `modRoot` selects the module, the source is the module plus `api`, and each module has its own dependency hash in `nix/go-modules.json`.
28. **Vendor mode would put the local `api` into every hash.** Then any `api` change invalidates the hash of every module using it. With `proxyVendor`, the dependency download is the module cache of external modules only; `api` comes from the source. A module without external dependencies has no hash (`null`), local ones included.
29. **The release tool needs Go** (`go mod edit` reads replace directives), so Go is one of its pinned tools. Linux runners have Go installed and hid the gap; the macOS runner showed it.
30. **cobra uses `--flags`**: every call site (Nix build, workflows) had to change from `-flag`.
31. **If the tools' own hash is stale, `nix run .#update` cannot build.** The weekly run never hits this (the tool builds before it updates anything); by hand, `go run ./tools/update go` in the development shell works without the hash.

