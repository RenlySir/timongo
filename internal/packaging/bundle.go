package packaging

const (
	// DefaultTiDBVersion is the enterprise packaging baseline requested for timongo.
	DefaultTiDBVersion = "v8.5.6"
	DefaultOS          = "linux"
	DefaultArch        = "amd64"
)

// BundleSpec describes a reproducible TiUP offline bundle for timongo and TiDB.
type BundleSpec struct {
	TiDBVersion     string   `json:"tidb_version"`
	TimongoVersion  string   `json:"timongo_version"`
	OS              string   `json:"os"`
	Arch            string   `json:"arch"`
	TiDBComponents  []string `json:"tidb_components"`
	TiUPComponents  []string `json:"tiup_components"`
	ToolkitIncludes []string `json:"toolkit_includes"`
}

// BundleLayout lists the stable paths inside the generated tarball.
type BundleLayout struct {
	Paths []string `json:"paths"`
}

// DefaultBundleSpec returns the default TiDB v8.5.6 Linux x86_64 package shape.
func DefaultBundleSpec() BundleSpec {
	return BundleSpec{
		TiDBVersion:    DefaultTiDBVersion,
		TimongoVersion: "dev",
		OS:             DefaultOS,
		Arch:           DefaultArch,
		TiDBComponents: []string{
			"tidb",
			"tikv",
			"pd",
			"tiflash",
			"prometheus",
			"grafana",
			"alertmanager",
			"node_exporter",
			"blackbox_exporter",
		},
		TiUPComponents: []string{
			"tiup",
			"cluster",
		},
		ToolkitIncludes: []string{
			"br",
			"cdc",
			"dumpling",
			"tidb-lightning",
			"sync-diff-inspector",
		},
	}
}

// IncludesComponent reports whether component is part of the offline TiUP mirror.
func (s BundleSpec) IncludesComponent(component string) bool {
	for _, got := range append(append([]string{}, s.TiDBComponents...), s.TiUPComponents...) {
		if got == component {
			return true
		}
	}
	return false
}

// Layout returns the stable on-disk layout for generated bundles.
func (s BundleSpec) Layout() BundleLayout {
	return BundleLayout{Paths: []string{
		"README.md",
		"bin/timongo-server",
		"manifest/timongo-bundle.json",
		"tiup/mirror",
		"tiup/components/timongo/timongo-component.tar.gz",
		"tiup/examples/topology.yaml",
		"tiup/templates/timongo.toml.tmpl",
		"tiup/templates/timongo.service.tmpl",
		"tiup/templates/haproxy.cfg.tmpl",
		"scripts/install-local-mirror.sh",
		"scripts/deploy-all.sh",
		"scripts/deploy-tidb-cluster.sh",
		"scripts/deploy-timongo.sh",
	}}
}

// Includes reports whether path is part of the layout.
func (l BundleLayout) Includes(path string) bool {
	for _, got := range l.Paths {
		if got == path {
			return true
		}
	}
	return false
}

// MirrorCloneArgs returns a tiup command line that clones TiDB v8.5.6 components.
func (s BundleSpec) MirrorCloneArgs(targetDir string) []string {
	return []string{"mirror", "clone", targetDir, s.TiDBVersion, "--os=" + s.OS, "--arch=" + s.Arch}
}

// PublishTimongoArgs returns a tiup command line that publishes timongo into a local mirror.
func (s BundleSpec) PublishTimongoArgs(tarball, privateKey string) []string {
	return []string{
		"mirror",
		"publish",
		"timongo",
		s.TimongoVersion,
		tarball,
		"timongo-server",
		"--standalone",
		"--os=" + s.OS,
		"--arch=" + s.Arch,
		"--key=" + privateKey,
		"--desc=timongo MongoDB 6.0 compatible gateway for TiDB " + s.TiDBVersion,
	}
}
