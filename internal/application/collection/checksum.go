package collection

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"
)

func contentChecksum(command ImportCollectionRunCommand) string {
	h := sha256.New()
	for _, value := range []string{
		string(command.Format),
		command.DataSourceID,
		command.NeighborhoodID,
		command.SourceRef,
		command.CollectedAt.UTC().Format(time.RFC3339Nano),
		string(command.Coverage),
	} {
		_, _ = io.WriteString(h, value)
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write(command.RawPayload)
	return hex.EncodeToString(h.Sum(nil))
}
