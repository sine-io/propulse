package fangjian

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
)

type FileArchive struct {
	root string
}

func NewFileArchive(root string) *FileArchive {
	return &FileArchive{root: filepath.Clean(root)}
}

func (a *FileArchive) Write(ctx context.Context, collection appcommunitymarket.CollectedCommunity) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if collection.Slug == "" || strings.Contains(collection.Slug, "..") || strings.ContainsAny(collection.Slug, `/\\`) {
		return "", errors.New("archive slug is invalid")
	}
	stamp := collection.Bundle.CollectedAt.UTC().Format("20060102T150405Z")
	parent := filepath.Join(a.root, stamp)
	finalDir := filepath.Join(parent, collection.Slug)
	if _, err := os.Stat(finalDir); err == nil {
		return "", fmt.Errorf("archive already exists: %s", finalDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp(parent, "."+collection.Slug+"-tmp-")
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(tempDir)
		}
	}()
	if err := os.Mkdir(filepath.Join(tempDir, "raw"), 0o700); err != nil {
		return "", err
	}

	files := make(map[string][]byte, len(collection.Raw)+4)
	bundleJSON, err := json.MarshalIndent(collection.Bundle, "", "  ")
	if err != nil {
		return "", err
	}
	files["bundle.json"] = append(bundleJSON, '\n')
	rawNames := make([]string, 0, len(collection.Raw))
	for key, raw := range collection.Raw {
		if strings.Contains(key, "..") || strings.ContainsAny(key, `/\\`) {
			return "", errors.New("raw archive key is invalid")
		}
		name := filepath.Join("raw", key+".json")
		var compact json.RawMessage
		if err := json.Unmarshal(raw, &compact); err != nil {
			return "", err
		}
		formatted, err := json.MarshalIndent(compact, "", "  ")
		if err != nil {
			return "", err
		}
		files[name] = append(formatted, '\n')
		rawNames = append(rawNames, name)
	}
	sort.Strings(rawNames)
	manifest := appcommunitymarket.ArchiveManifest{
		SchemaVersion: collection.Bundle.SchemaVersion, CollectedAt: collection.Bundle.CollectedAt,
		CommunityID: collection.Bundle.Community.SourceCommunityID, CommunityName: collection.Bundle.Community.CommunityName,
		Files:     map[string]string{"bundle": "bundle.json", "checksums": "SHA256SUMS", "result": "result.json"},
		Endpoints: append([]string(nil), collection.Endpoints...),
	}
	for _, name := range rawNames {
		manifest.Files["raw."+strings.TrimSuffix(filepath.Base(name), ".json")] = filepath.ToSlash(name)
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	files["manifest.json"] = append(manifestJSON, '\n')
	resultJSON, _ := json.MarshalIndent(map[string]any{
		"status": "complete", "listingCount": len(collection.Bundle.Listings),
		"transactionCount": len(collection.Bundle.Transactions), "adjustmentCount": len(collection.Bundle.Adjustments),
	}, "", "  ")
	files["result.json"] = append(resultJSON, '\n')

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	checksumLines := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, files[name], 0o600); err != nil {
			return "", err
		}
		hash := sha256.Sum256(files[name])
		checksumLines = append(checksumLines, hex.EncodeToString(hash[:])+"  "+filepath.ToSlash(name))
	}
	checksums := []byte(strings.Join(checksumLines, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(tempDir, "SHA256SUMS"), checksums, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tempDir, finalDir); err != nil {
		return "", err
	}
	committed = true
	return finalDir, nil
}
