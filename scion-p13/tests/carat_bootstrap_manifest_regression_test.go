package hub

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/storage"
)

// TestCARATBootstrapWritesHarnessManifest verifies that a real Hub bootstrap,
// not a filesystem imitation, produces a usable manifest on the in-memory
// storage backend. This test is isolated from the live Scion Hub.
func TestCARATBootstrapWritesHarnessManifest(t *testing.T) {
	srv, _, stor := testTemplateBootstrapServer(t)
	ctx := context.Background()

	br := testBundledResource(storage.ResourceKindHarnessConfig, "carat-bootstrap-test", map[string]string{
		"config.yaml":  "harness: claude\nimage: test:latest\nuser: scion\n",
		"home/.bashrc": "# test fixture",
	})
	rs := srv.harnessConfigStore("claude")
	src := NewFSResourceSource(br)
	result, err := rs.BootstrapSource(ctx, src, BootstrapOptions{
		OverwritePolicy: OverwriteBuiltinManaged,
	})
	if err != nil {
		t.Fatal("bootstrap:", err)
	}
	if result.Created != 1 {
		t.Fatalf("bootstrap created=%d; want 1", result.Created)
	}
	hc, err := srv.store.GetHarnessConfigBySlug(ctx, "carat-bootstrap-test", "global", "")
	if err != nil {
		t.Fatal("lookup:", err)
	}
	path := hc.StoragePath + "/manifest.json"
	exists, err := stor.Exists(ctx, path)
	if err != nil || !exists {
		t.Fatalf("bootstrap manifest missing: exists=%v err=%v", exists, err)
	}
	reader, _, err := stor.Download(ctx, path)
	if err != nil {
		t.Fatal("download manifest:", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 1048576))
	if err != nil {
		t.Fatal("read manifest:", err)
	}
	var manifest struct {
		Version string `json:"version"`
		Files   []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal("invalid manifest JSON:", err)
	}
	if manifest.Version != "1.0" || len(manifest.Files) != 2 {
		t.Fatalf("unexpected manifest metadata: version=%q count=%d", manifest.Version, len(manifest.Files))
	}
	report, err := rs.ValidateStorage(ctx, harnessConfigToRecord(hc))
	if err != nil {
		t.Fatal("validate:", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("expected validation PASS, got %d issues", len(report.Issues))
	}

	// An update invokes storage reconciliation. Manifest must survive it.
	result, err = rs.BootstrapSource(ctx, src, BootstrapOptions{
		OverwritePolicy: OverwriteAlways, Force: true,
	})
	if err != nil {
		t.Fatal("resync:", err)
	}
	_ = result
	exists, err = stor.Exists(ctx, path)
	if err != nil || !exists {
		t.Fatalf("resync removed manifest: exists=%v err=%v", exists, err)
	}
	hc, err = srv.store.GetHarnessConfigBySlug(ctx, "carat-bootstrap-test", "global", "")
	if err != nil {
		t.Fatal("lookup after resync:", err)
	}
	report, err = rs.ValidateStorage(ctx, harnessConfigToRecord(hc))
	if err != nil || len(report.Issues) != 0 {
		t.Fatalf("post-resync validation failed: err=%v issues=%v", err, report.Issues)
	}
}