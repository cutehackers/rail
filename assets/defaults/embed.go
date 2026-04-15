package defaults

import "embed"

// FS exposes the canonical embedded harness defaults from this directory.
//
//go:embed actors/* rubrics/* rules/* supervisor/* templates/*
var FS embed.FS
