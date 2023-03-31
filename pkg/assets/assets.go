package assets

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-contrib/static"
)

//go:embed static/*
var StaticContent embed.FS

func ServeEmbedContent() (static.ServeFileSystem, error) {
	fsys, err := fs.Sub(StaticContent, "static") // matches the path in `go:embed` above
	if err != nil {
		return nil, err
	}
	return StaticContentFileSystem{
		FileSystem: http.FS(fsys),
	}, nil
}

type StaticContentFileSystem struct {
	http.FileSystem
}

func (e StaticContentFileSystem) Exists(_ string, path string) bool {
	_, err := e.Open(path)
	return err == nil
}
