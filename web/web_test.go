package web

import (
	"io/fs"
	"testing"
)

func TestEmbeddedWebContainsIndex(t *testing.T) {
	embedded := Embedded()
	if _, err := fs.Stat(embedded, "index.html"); err != nil {
		t.Fatalf("index.html missing from embedded web fs: %v", err)
	}
}
