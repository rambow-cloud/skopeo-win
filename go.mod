module go.podman.io/skopeo

// Minimum required golang version
go 1.27.4

// Warning: Ensure the "go" and "toolchain" versions match exactly to prevent unwanted auto-updates

require (
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/ProtonMail/go-crypto v1.5.2
	github.com/containers/ocicrypt v1.3.2
	github.com/docker/distribution v2.8.3+incompatible
	github.com/moby/sys/capability v0.5.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.2-0.20260226102121-a4c6ade7bb82
	github.com/opencontainers/image-tools v1.0.0-rc3
	github.com/sirupsen/logrus v1.10.2
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.1.0
	github.com/stretchr/testify v1.12.1
	go.podman.io/common v0.70.0
	go.podman.io/image/v5 v5.42.1
	go.podman.io/storage v1.65.1
	go.yaml.in/yaml/v3 v3.0.6
	golang.org/x/sys v0.50.0
	golang.org/x/term v0.48.0
)

require (
	cyphar.com/go-pathrs v0.2.6 // indirect
	dario.cat/mergo v1.0.2 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.5 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.19.1 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/coreos/go-oidc/v3 v3.21.0 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20261011152517-32a48195b9b2 // indirect
	github.com/cyphar/filepath-securejoin v0.7.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker-credential-helpers v0.9.13 // indirect
	github.com/docker/go-connections v0.8.2 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.9 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/go-containerregistry v0.22.1 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.2-0.20250313123807-1ee6e1a1957a // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.20.4 // indirect
	github.com/klauspost/pgzip v1.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/mattn/go-sqlite3 v1.14.53 // indirect
	github.com/miekg/pkcs11 v1.1.2 // indirect
	github.com/mistifyio/go-zfs/v4 v4.0.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/moby/api v1.57.0 // indirect
	github.com/moby/moby/client v0.6.3 // indirect
	github.com/moby/sys/mountinfo v0.7.3 // indirect
	github.com/moby/sys/user v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/opencontainers/image-spec/schema v0.0.0-20250717171153-ab80ff15c2dd // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/opencontainers/selinux v1.15.1 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/proglottis/gpgme v0.1.6 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.11.1 // indirect
	github.com/sigstore/fulcio v1.10.0 // indirect
	github.com/sigstore/protobuf-specs v0.5.2 // indirect
	github.com/sigstore/sigstore v1.11.3 // indirect
	github.com/smallstep/pkcs7 v0.2.3 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20230803200340-78284954bff6 // indirect
	github.com/sylabs/sif/v2 v2.24.2 // indirect
	github.com/tchap/go-patricia/v2 v2.3.3 // indirect
	github.com/ulikunitz/xz v0.5.17 // indirect
	github.com/varlink/go v0.4.1-0.20261015173642-c935be0c53f7 // indirect
	github.com/vbatts/tar-split v0.12.3 // indirect
	github.com/vbauerster/cupwriter v0.0.5 // indirect
	github.com/vbauerster/mpb/v8 v8.16.3 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/oauth2 v0.38.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261110000943-09e21b237f2c // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261110000943-09e21b237f2c // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
