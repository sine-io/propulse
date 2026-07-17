package fangjian

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
)

func TestClientRetriesTemporaryStatusAndKeepsCredentialsInHeadersOnly(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" || request.Header.Get("ak") != "ak-secret" || request.Header.Get("Version") != "1.0.22" {
			t.Errorf("credential headers were not set")
		}
		if calls.Add(1) == 1 {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = writer.Write([]byte(`{"code":200,"data":{},"description":"ok"}`))
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{
		BaseURL: server.URL, Authorization: "Bearer secret", AK: "ak-secret", Version: "1.0.22",
		MinInterval: time.Nanosecond, MaxAttempts: 3,
	}, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	body, err := client.Get(context.Background(), "/test")
	if err != nil || !strings.Contains(string(body), `"code":200`) || calls.Load() != 2 {
		t.Fatalf("Get() body/error/calls = %s/%v/%d", body, err, calls.Load())
	}
}

func TestClientDoesNotRetryAuthenticationFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	client, _ := NewClient(ClientConfig{BaseURL: server.URL, Authorization: "secret", AK: "ak", Version: "v", MinInterval: time.Nanosecond, MaxAttempts: 3}, server.Client())
	_, err := client.Get(context.Background(), "/test")
	if err == nil || calls.Load() != 1 {
		t.Fatalf("Get() error/calls = %v/%d", err, calls.Load())
	}
}

func TestFileArchiveWritesManifestChecksumsAndNoRequestHeaders(t *testing.T) {
	root := t.TempDir()
	archive := NewFileArchive(root)
	collectedAt := time.Date(2026, 7, 17, 1, 2, 3, 0, time.UTC)
	path, err := archive.Write(context.Background(), appcommunitymarket.CollectedCommunity{
		Slug: "mingquan",
		Bundle: appcommunitymarket.FangjianBundle{
			SchemaVersion: appcommunitymarket.FangjianBundleSchemaVersion, CollectedAt: collectedAt,
			Community: domaincommunitymarket.SnapshotData{
				SourceCommunityID: "community", CommunityName: "鸣泉花园", CityCode: "120100", CityName: "天津市",
				DistrictCode: "120111", DistrictName: "西青区", BlockCode: "block", BlockName: "梅江南",
				Latitude: 39, Longitude: 117, Analysis: []byte(`{}`), Surroundings: []byte(`{}`), CityContext: []byte(`{}`),
			},
			Listings: []appcommunitymarket.MarketListing{}, Transactions: []appcommunitymarket.MarketTransaction{},
			Adjustments: []appcommunitymarket.ListingAdjustment{}, Quality: appcommunitymarket.BundleQuality{Status: "complete", Warnings: []string{}},
		},
		Raw:       map[string]json.RawMessage{"basic-info": json.RawMessage(`{"code":200,"data":{}}`)},
		Endpoints: []string{"/esf/ex/basicInfo/community"},
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	for _, name := range []string{"bundle.json", "manifest.json", "result.json", "SHA256SUMS", "raw/basic-info.json"} {
		if _, err := os.Stat(filepath.Join(path, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	var contents strings.Builder
	_ = filepath.WalkDir(path, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		body, err := os.ReadFile(filePath)
		if err == nil {
			contents.Write(body)
		}
		return err
	})
	for _, forbidden := range []string{"Authorization", "Bearer secret", "ak-secret"} {
		if strings.Contains(contents.String(), forbidden) {
			t.Fatalf("archive contains forbidden header material %q", forbidden)
		}
	}
}
