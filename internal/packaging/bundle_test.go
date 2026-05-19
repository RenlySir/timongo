package packaging

import (
	"strings"
	"testing"
)

func TestDefaultBundleSpecTargetsTiDB856(t *testing.T) {
	spec := DefaultBundleSpec()

	if spec.TiDBVersion != "v8.5.6" {
		t.Fatalf("TiDBVersion = %q, want v8.5.6", spec.TiDBVersion)
	}
	if spec.OS != "linux" {
		t.Fatalf("OS = %q, want linux", spec.OS)
	}
	if spec.Arch != "amd64" {
		t.Fatalf("Arch = %q, want amd64", spec.Arch)
	}
	for _, component := range []string{"tidb", "tikv", "pd", "tiflash", "tiup", "cluster", "prometheus", "grafana", "alertmanager", "node_exporter", "blackbox_exporter"} {
		if !spec.IncludesComponent(component) {
			t.Fatalf("expected required component %q", component)
		}
	}
}

func TestBundleLayoutIncludesTiDBMirrorAndTimongoAssets(t *testing.T) {
	layout := DefaultBundleSpec().Layout()

	for _, want := range []string{
		"tiup/mirror",
		"bin/timongo-server",
		"tiup/templates/timongo.toml.tmpl",
		"tiup/templates/timongo.service.tmpl",
		"tiup/examples/topology.yaml",
		"scripts/deploy-all.sh",
		"scripts/deploy-timongo.sh",
		"manifest/timongo-bundle.json",
	} {
		if !layout.Includes(want) {
			t.Fatalf("bundle layout missing %q", want)
		}
	}
}

func TestTiUPCloneArgsArePinnedToTiDB856(t *testing.T) {
	spec := DefaultBundleSpec()
	args := spec.MirrorCloneArgs("/opt/timongo/tiup/mirror")
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"mirror clone /opt/timongo/tiup/mirror v8.5.6",
		"--os=linux",
		"--arch=amd64",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("clone args %q missing %q", joined, want)
		}
	}
}

func TestTiUPPublishArgsDeclareTimongoStandaloneComponent(t *testing.T) {
	spec := DefaultBundleSpec()
	args := spec.PublishTimongoArgs("dist/timongo-component.tar.gz", "keys/private.json")
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"mirror publish timongo",
		"dist/timongo-component.tar.gz",
		"timongo-server",
		"--standalone",
		"--os=linux",
		"--arch=amd64",
		"--key=keys/private.json",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("publish args %q missing %q", joined, want)
		}
	}
}
