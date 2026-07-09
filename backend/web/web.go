package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embedded embed.FS

func Embedded() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err)
	}

	return sub
}
