#!/usr/bin/env bash
set -euo pipefail

cat <<'EOF' >&2
scripts/install_skill.sh is deprecated.

Rail packaged installs now bundle the Codex skill automatically.
Use a packaged install instead:

  brew install rail

For local source checkouts, the bundled skill source lives under:

  assets/skill/Rail/

This script no longer creates ~/.codex skill symlinks from a checkout.
EOF

exit 1
