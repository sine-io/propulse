package review

import (
	"strings"
	"testing"
)

func TestCreateNoteInputValidate(t *testing.T) {
	validNeighborhoodID := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name    string
		input   CreateNoteInput
		wantErr bool
	}{
		{name: "valid review", input: CreateNoteInput{UserID: "u1", Kind: KindReview, Content: "本周复盘"}},
		{name: "valid viewing note", input: CreateNoteInput{UserID: "u1", NeighborhoodID: &validNeighborhoodID, Kind: KindViewingNote, Content: "看房记录"}},
		{name: "missing user", input: CreateNoteInput{Kind: KindReview, Content: "x"}, wantErr: true},
		{name: "invalid kind", input: CreateNoteInput{UserID: "u1", Kind: Kind("other"), Content: "x"}, wantErr: true},
		{name: "empty content", input: CreateNoteInput{UserID: "u1", Kind: KindReview, Content: ""}, wantErr: true},
		{name: "content too long by unicode characters", input: CreateNoteInput{UserID: "u1", Kind: KindReview, Content: strings.Repeat("界", MaxContentRunes+1)}, wantErr: true},
		{name: "invalid neighborhood id", input: CreateNoteInput{UserID: "u1", NeighborhoodID: stringPointer("not-a-uuid"), Kind: KindReview, Content: "x"}, wantErr: true},
		{name: "noncanonical neighborhood id", input: CreateNoteInput{UserID: "u1", NeighborhoodID: stringPointer(strings.ReplaceAll(validNeighborhoodID, "-", "")), Kind: KindReview, Content: "x"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func stringPointer(value string) *string { return &value }
