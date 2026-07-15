package review

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{name: "trims unicode whitespace", content: " \t本周复盘\n ", want: "本周复盘"},
		{name: "accepts unicode at limit", content: strings.Repeat("界", MaxContentRunes), want: strings.Repeat("界", MaxContentRunes)},
		{name: "rejects whitespace", content: " \t\n", wantErr: true},
		{name: "rejects unicode beyond limit", content: strings.Repeat("界", MaxContentRunes+1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeContent(tt.content)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidNote) {
					t.Fatalf("NormalizeContent() error = %v, want ErrInvalidNote", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeContent() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
