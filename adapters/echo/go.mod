module github.com/anverse/spike-nix-release/adapters/echo

go 1.26.0

require (
	github.com/anverse/spike-nix-release/api v0.0.0
	github.com/google/uuid v1.6.0
)

replace github.com/anverse/spike-nix-release/api => ../../api
