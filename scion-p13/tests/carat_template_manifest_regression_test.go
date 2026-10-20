package hub

import (
	"context"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/storage"
)

// TestCARATTemplateBootstrapManifestCompatibility ensures that the shared
// uploader change does not regress template resources (not just Harness).
func TestCARATTemplateBootstrapManifestCompatibility(t *testing.T) {
	srv, _, stor := testTemplateBootstrapServer(t)
	ctx := context.Background()
	br := testBundledResource(storage.ResourceKindTemplate, "carat-template-manifest", map[string]string{
		"scion-agent.yaml": "harness: claude\n",
		"home/.bashrc":     "# safe synthetic data",
	})
	src := NewFSResourceSource(br)
	rs := srv.templateStore()

	if _, err := rs.BootstrapSource(ctx, src, BootstrapOptions{
		OverwritePolicy: OverwriteBuiltinManaged,
	}); err != nil {
		t.Fatal("bootstrap template:", err)
	}
	tmpl, err := srv.store.GetTemplateBySlug(ctx, "carat-template-manifest", "global", "")
	if err != nil {
		t.Fatal("lookup template:", err)
	}
	manifest := tmpl.StoragePath + "/manifest.json"
	exists, err := stor.Exists(ctx, manifest)
	if err != nil || !exists {
		t.Fatalf("template manifest absent: exists=%v err=%v", exists, err)
	}
	validation, err := rs.ValidateStorage(ctx, templateToRecord(tmpl))
	if err != nil || len(validation.Issues) != 0 {
		t.Fatalf("template validate failed: err=%v issues=%v", err, validation.Issues)
	}

	if _, err := rs.BootstrapSource(ctx, src, BootstrapOptions{
		OverwritePolicy: OverwriteAlways, Force: true,
	}); err != nil {
		t.Fatal("template resync:", err)
	}
	exists, err = stor.Exists(ctx, manifest)
	if err != nil || !exists {
		t.Fatalf("template manifest lost during resync: exists=%v err=%v", exists, err)
	}
}