# Spike 0011: build and release with Nix, without GoReleaser

| | |
|---|---|
| Status | Part A done (2026-10-14, darwin/arm64, local). Part B (GitHub) pending: needs the repository `anverse/spike-nix-release`. |
| Question | Can a Nix flake build every artifact of a monorepo (a CLI for three operating systems, multi-platform images, a Helm chart, Homebrew and Scoop manifests), with Taskfile as the only entry point and small CLIs publishing one GitHub release per component tag, identically on a laptop and in CI? |
| Feeds into | [ADR-0023](../../spec/adr/0023-nix-as-the-single-build-system.md) (Nix as the single build system), [ADR-0024](../../spec/adr/0024-releases-per-component.md) (releases per component), [ADR-0019](../../spec/adr/0019-production-like-development-environment.md) and [development.md](../../spec/development.md) (environment, e2e as Helm tests, CI), [milestone v0.0](../../spec/milestone/v0.0.md), roadmap open question 28 |
| Builds on | Spike 0002 (k3d with gVisor) |
| Versions | nixpkgs unstable (2026-10): distroless `static-debian13:nonroot` (pinned by digest), Go 1.26.7, regclient 0.11.6, git-cliff 2.14.2, Helm 4.3.0, helmfile 1.8.0, k3d 5.9.0, go-task 3.53.1, nix-update 1.16.0, gh 2.101.0; k3s v1.34.12-k3s1; gVisor 20261005; Nix 2.34 |

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

```
flake.nix                 dev shell; one package per artifact, listed by name; lib for the tasks
versions.json             version per component (written by `task release`)
pins.json                 external artifacts: distroless digest, gVisor release, k3s, with hashes
project.json              repository, registry, tap, bucket, which component is "latest"
nix/components.nix        kind and release-note paths per component
nix/go.nix                Go builds: a command for one platform, the e2e test binary
nix/cli.nix               a CLI's release: archives per platform, checksums, formula, Scoop manifest
nix/service.nix           a service's binaries, image and chart
nix/image.nix             an image: distroless base + our binary, both platforms, built by regctl
nix/distroless.nix        the pinned distroless base, fetched once
nix/gvisor.nix            gVisor for the development cluster (from spike 0002)
packaging/                Homebrew formula and Scoop manifest templates
charts/spiked/            hand-written chart: only used values, values.schema.json
cliff.toml                git-cliff: plain commit list, no convention
Taskfile.yml              everyday tasks (lint, test, check, e2e); includes taskfiles/
taskfiles/release.yml     cut, dry run, publish
taskfiles/dev.yml         development cluster, images to the local registry, deploy
taskfiles/update.yml      update flake.lock, Go modules, pins.json
taskfiles/spike.yml       this spike's own checks
dev/                      k3d config, containerd template (gVisor), helmfile (local, ci), cluster-base chart
test/e2e/                 e2e suite (Go tests) and its chart: one Helm test pod
.github/                  ci (3 runners + digest comparison), e2e (amd64, arm64), release, install, update; dependabot
```

## How to run

```sh
nix develop
task lint test check                     # vet, gofmt, chart lint, unit tests, every artifact
task release:dry-run COMPONENT=spikectl  # archives, manifests, notes into dist/
task release:dry-run COMPONENT=spiked    # image layout, chart, notes into dist/
task e2e                                 # k3d + gVisor + registry, helmfile, Helm tests
task spike:verify-notes                  # releases and notes in a throwaway repository
task update                              # move every pin to the newest version
task dev:down
```

Part B: copy the directory into a clone of `anverse/spike-nix-release` and push it, then `task release COMPONENT=spikectl VERSION=0.1.0 MESSAGE="…"` and the same for `spiked`. Afterwards: reset the spike commits in `anverse/homebrew-tap` and `anverse/scoop-bucket`, delete the repository and its two ghcr.io packages (deleting a repository keeps its packages).

## Results (part A)

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
