//go:build embeddist

// Package web anchors the compiled frontend so the server binary can embed it
// (go:embed). A binary-only upgrade then ships a matching frontend instead of
// pairing a new backend with a stale web/dist on disk.
//
// This file only participates in builds with the `embeddist` tag (make prod /
// make upgrade, which always build the frontend first). Default builds use the
// stub in embed_stub.go so a manually deleted web/dist never breaks
// `go build ./...` / `go test ./...`.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
