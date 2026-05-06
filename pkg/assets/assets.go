package assets

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var StaticContent embed.FS

func ServeEmbedContent() (fs.FS, error) {
	return fs.Sub(StaticContent, "static")
}
