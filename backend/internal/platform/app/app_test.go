package app

import "testing"

func TestNormalizeModeAcceptsDocumentedModes(t *testing.T) {
	for _, args := range [][]string{
		{"serve"},
		{"api"},
		{"worker"},
		{"scheduler"},
		{"migrate", "up"},
		{"migrate", "down"},
	} {
		mode, err := NormalizeMode(args)
		if err != nil {
			t.Fatalf("NormalizeMode(%v) error = %v", args, err)
		}
		if mode == "" {
			t.Fatalf("NormalizeMode(%v) returned empty mode", args)
		}
	}
}
