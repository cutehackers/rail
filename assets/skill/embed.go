package skill

import "embed"

// FS exposes the packaged Rail skill assets for release packaging helpers.
//
//go:embed Rail/*.md Rail/references/*.md
var FS embed.FS
