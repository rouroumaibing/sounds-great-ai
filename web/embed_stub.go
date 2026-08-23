//go:build !embeddist

// Package web anchors the compiled frontend so the server binary can embed it.
// This is the default (no-tag) variant: an empty filesystem. SPAHandler then
// serves from web/dist on disk only, and go:embed never requires the dist
// directory to exist — source users may freely `rm -rf web/dist`.
package web

import "embed"

// DistFS is the zero-value embed.FS: contains no files.
var DistFS embed.FS
