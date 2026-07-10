package collection

import (
	"testing"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestContentChecksumIsStableForExactReplay(t *testing.T) {
	command := checksumCommand()

	first := contentChecksum(command)
	second := contentChecksum(command)

	const want = "0024fe00c3bc45e810a47b1c2a12570ee6bf5446e7940efdb9f9537ffb62561e"
	if first != want {
		t.Fatalf("contentChecksum() = %q, want %q", first, want)
	}
	if second != first {
		t.Fatalf("contentChecksum() replay = %q, want %q", second, first)
	}
}

func TestContentChecksumChangesWithRawBytesOrImportMetadata(t *testing.T) {
	base := checksumCommand()
	baseChecksum := contentChecksum(base)

	withDifferentRawBytes := base
	withDifferentRawBytes.RawPayload = []byte(`{"records":2,"changed":true}`)
	if got := contentChecksum(withDifferentRawBytes); got == baseChecksum {
		t.Fatal("contentChecksum did not change when raw bytes changed")
	}

	withDifferentMetadata := base
	withDifferentMetadata.SourceRef = "weekly-2026-07-10"
	if got := contentChecksum(withDifferentMetadata); got == baseChecksum {
		t.Fatal("contentChecksum did not change when import metadata changed")
	}
}

func checksumCommand() ImportCollectionRunCommand {
	return ImportCollectionRunCommand{
		DataSourceID:   "11111111-1111-1111-1111-111111111111",
		NeighborhoodID: "22222222-2222-2222-2222-222222222222",
		SourceRef:      "weekly-2026-07-09",
		CollectedAt:    time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Coverage:       domainneighborhood.CoverageFull,
		Format:         ImportFormatJSON,
		RawPayload:     []byte(`{"records":2}`),
		RawContentType: "application/json",
	}
}
