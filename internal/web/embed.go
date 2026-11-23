// Package web holds the embedded frontend assets.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:assets index.html admin.html
var content embed.FS

// FS returns the asset filesystem rooted at the web directory.
func FS() fs.FS { return content }

// Index is the public navigation shell.
var Index = mustRead("index.html")

// Admin is the management console shell.
var Admin = mustRead("admin.html")

func mustRead(name string) []byte {
	data, err := content.ReadFile(name)
	if err != nil {
		panic("web: missing embedded file " + name + ": " + err.Error())
	}
	return data
}
