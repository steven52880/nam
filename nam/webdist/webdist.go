// Package webdist embeds the built frontend (web/ → npm run build → nam/webdist/dist).
package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the frontend files rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // embed path is fixed at compile time
	}
	return sub
}
