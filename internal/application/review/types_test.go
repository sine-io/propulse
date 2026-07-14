package review

import (
	"strings"
	"testing"
)

func TestCreateNoteInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateNoteInput
		wantErr bool
	}{
		{
			name:  "valid review",
			input: CreateNoteInput{UserID: "u1", Kind: KindReview, Content: "本周复盘"},
		},
		{
			name:  "valid viewing note",
			input: CreateNoteInput{UserID: "u1", Kind: KindViewingNote, Content: "看房记录"},
		},
		{
			name:    "missing user",
			input:   CreateNoteInput{Kind: KindReview, Content: "x"},
			wantErr: true,
		},
		{
			name:    "invalid kind",
			input:   CreateNoteInput{UserID: "u1", Kind: Kind("other"), Content: "x"},
			wantErr: true,
		},
		{
			name:    "empty content",
			input:   CreateNoteInput{UserID: "u1", Kind: KindReview, Content: ""},
			wantErr: true,
		},
		{
			name:    "content too long",
			input:   CreateNoteInput{UserID: "u1", Kind: KindReview, Content: strings.Repeat("a", maxContentLength+1)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}
