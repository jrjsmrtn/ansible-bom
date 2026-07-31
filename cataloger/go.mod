module github.com/jrjsmrtn/ansible-bom/cataloger

go 1.26.5

// The cataloger consumes the parsers from the parent module. A replace keeps it building against
// the working tree rather than a published version, which matters while content/ is 0.x and its
// API is still free to move (ADR-0008).
//
// KNOWN LIMITATION: this module is NOT independently consumable, by design and for now.
// A replace directive is ignored when a module is used as a dependency, so `go get` of this
// module would try to resolve the placeholder version below and fail. No published tag of the
// parent contains the content package either — it was internal/content until 2026-07-31, so
// neither v0.1.0 nor v0.1.1 would satisfy the requirement.
//
// This is tolerable because the destination for this code is syft's own tree, where neither the
// replace nor the parent dependency travels with it. Resolve by dropping the replace and
// requiring a real parent version once one is tagged that contains content/.
replace github.com/jrjsmrtn/ansible-bom => ../

require (
	github.com/anchore/syft v1.50.0
	github.com/jrjsmrtn/ansible-bom v0.0.0-00010101000000-000000000000
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/acobaugh/osrelease v0.1.0 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/anchore/clio v0.1.1 // indirect
	github.com/anchore/fangs v0.1.1 // indirect
	github.com/anchore/go-homedir v0.1.1 // indirect
	github.com/anchore/go-logger v0.1.1 // indirect
	github.com/anchore/go-sync v0.1.1 // indirect
	github.com/anchore/packageurl-go v0.2.0 // indirect
	github.com/anchore/stereoscope v0.3.0 // indirect
	github.com/becheran/wildmatch-go v1.0.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/docker/cli v29.6.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.5 // indirect
	github.com/docker/go-connections v0.7.0 // indirect
	github.com/facebookincubator/nvdtools v0.1.5 // indirect
	github.com/felixge/fgprof v0.9.5 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/github/go-spdx/v2 v2.7.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gohugoio/hashstructure v0.6.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.21.7 // indirect
	github.com/google/licensecheck v0.3.1 // indirect
	github.com/google/pprof v0.0.0-20250317173921-a4b03ec1a45e // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gookit/color v1.6.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/klauspost/compress v1.19.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pborman/indent v1.2.1 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/pkg/profile v1.7.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/scylladb/go-set v1.0.3-0.20200225121959-cc7b2070d91e // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/sylabs/squashfs v1.0.6 // indirect
	github.com/therootcompany/xz v1.0.1 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/wagoodman/go-partybus v0.0.0-20230516145632-8ccac152c651 // indirect
	github.com/wagoodman/go-progress v0.0.0-20260303201901-10176f79b2c0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)
