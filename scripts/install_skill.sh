#!/usr/bin/env bash
set -euo pipefail

cat <<'EOF' >&2
scripts/install_skill.sh is deprecated.

This source checkout is not a supported end-user install path.
Rail packaged releases bundle the Codex skill with the installed product.

Install Rail from the Homebrew tap:

  brew install cutehackers/rail/rail

Release artifacts are published through GitHub Releases:

  https://github.com/cutehackers/rail/releases

The Homebrew formula in this repository is release-packaging material:

  packaging/homebrew/rail.rb

Rail now registers the active Codex user skill copy during project setup:

  rail init
  rail doctor
  rail install-codex-skill --repair

Contributor checks for the packaged skill layout:

  go test ./internal/install -v
  brew install --build-from-source ./packaging/homebrew/rail.rb

For local source checkouts, the bundled skill source lives under:

  assets/skill/Rail/

This script no longer creates checkout-coupled skill links.
EOF

exit 1
