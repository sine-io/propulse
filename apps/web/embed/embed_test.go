package webembed

import (
	"io/fs"
	"testing"
)

func TestEmbeddedWebContainsAllStaticRoutes(t *testing.T) {
	embedded := Embedded()
	for _, name := range []string{
		"index.html",
		"calculator.html",
		"data.html",
		"data/imports/_.html",
		"neighborhoods.html",
		"action-window.html",
		"methods.html",
		"templates.html",
		"watchlist.html",
		"icon.svg",
	} {
		if _, err := fs.Stat(embedded, name); err != nil {
			t.Errorf("%s missing from embedded web fs: %v", name, err)
		}
	}
}
