#!/usr/bin/env bash
set -euo pipefail

cat <<'EOF' >&2
scripts/install_skill.sh is deprecated.

This source checkout is not a supported end-user install path.
Rail packaged releases bundle the Codex skill as part of release packaging.
Use the published release or tap instructions for the version you want.

The Homebrew formula in this repository is release-packaging material:

  packaging/homebrew/rail.rb

For local source checkouts, the bundled skill source lives under:

  assets/skill/Rail/

This script no longer creates checkout-coupled skill symlinks.
EOF

exit 1
