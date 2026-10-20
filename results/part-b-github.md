# Part B on GitHub, 2026-10-15

Repository `anverse/spike-nix-release` (public), tap `anverse/homebrew-tap`,
bucket `anverse/scoop-bucket`. Run ids refer to that repository's Actions.

## CI (`ci`, `e2e`)

| Run | Result | Notes |
|---|---|---|
| ci 38708483696 | verify ok on ubuntu-24.04, ubuntu-24.04-arm, macos-15; compare **failed** | Artifacts differed between Linux and macOS: Nix's fixup (strip, re-signing) and nixpkgs Go's Nix store paths for tzdata, iana-etc, mailcap |
| ci 38710201580 | ok, compare ok | After building with Go without those patches and without fixup: bit-identical on all three runners. 5 to 6 min (Go toolchain rebuilt, cache cold) |
| e2e 38708483678 | ok on amd64 and arm64 | gVisor in k3d on GitHub's Linux runners, both architectures: 1.5 to 2.5 min |
| e2e 38712027631, 38713811060 | one architecture failed each | Test pod refused by the Service right after the Deployment became ready; fixed by retrying in the test |

## Releases (`release`, `install`)

| Tag | Run | Result |
|---|---|---|
| spikectl/v0.1.0 | 38710989263 | Release, archives, formula and manifest ok; Homebrew ok on macos-15, macos-15-intel, ubuntu-24.04, ubuntu-24.04-arm; Scoop ok on windows-2025, windows-11-arm; `releases/latest` download ok. Nix install **failed**: `nix run` started `bin/spike` (shared pname) |
| spiked/v0.1.0 | 38712064479 | Image and chart pushed to ghcr.io, release with digest. Pulling the image **failed**: new organization packages are private |
| spikectl/v0.1.1 | 38712761268 | Everything ok, Nix install included (3 to 4 min: builds the Go toolchain). Notes list only the one spikectl commit since 0.1.0; latest moved to 0.1.1 |

Notes of 0.1.0 started with the release commit's message: actions/checkout
turns annotated tags into lightweight tags. Fixed by fetching the tag.
Signed tags put their signature into the notes; fixed by using subject and
body only.

## Updates (`update`)

| Run | Result |
|---|---|
| update 38713817180 | 4 min. `task update` moved nixpkgs and k3s (v1.34.12 to v1.36.5 on `stable`); Go modules, distroless and gVisor were current. Opened PR #2 and dispatched ci (38714283879, ok) and e2e (38714286837, ok on k3s v1.36.5). The PR's own `pull_request` runs wait with `action_required`, and the dispatched runs do not appear as checks on the PR |

Dependabot opened PR #1 for the actions (five updates, Node 20 deprecation).
