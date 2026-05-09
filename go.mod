module code.gitea.io/gitea

go 1.26.3

// rfc5280 said: "The serial number is an integer assigned by the CA to each certificate."
// But some CAs use negative serial number, just relax the check. related:
// Default TLS cert uses negative serial number #895 https://github.com/microsoft/mssql-docker/issues/895
godebug x509negativeserial=1

require (
	code.gitea.io/actions-proto-go v0.4.1
	code.gitea.io/sdk/gitea v0.24.1
	codeberg.org/gusted/mcaptcha v0.0.0-20220723083913-4f3072e1d570
	connectrpc.com/connect v1.19.2
	gitea.com/gitea/runner v1.0.0
	gitea.com/go-chi/binding v0.0.0-20260414111559-654cea7ac60a
	gitea.com/go-chi/cache v0.2.1
	gitea.com/go-chi/captcha v0.0.0-20240315150714-fb487f629098
	gitea.com/go-chi/session v0.0.0-20251124165456-68e0254e989e
	gitea.com/lunny/dingtalk_webhook v0.0.0-20171025031554-e3534c89ef96
	gitea.com/lunny/levelqueue v0.4.2-0.20230414023320-3c0159fe0fe4
	github.com/42wim/httpsig v1.2.4
	github.com/42wim/sshsig v0.0.0-20260317195500-b9f38cf0d432
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.20.0
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.3
	github.com/Azure/go-ntlmssp v0.1.1
	github.com/ProtonMail/go-crypto v1.4.1
	github.com/PuerkitoBio/goquery v1.12.0
	github.com/SaveTheRbtz/zstd-seekable-format-go/pkg v0.8.0
	github.com/alecthomas/chroma/v2 v2.24.1
	github.com/aws/aws-sdk-go-v2/credentials v1.19.16
	github.com/aws/aws-sdk-go-v2/service/codecommit v1.33.14
	github.com/blakesmith/ar v0.0.0-20190502131153-809d4375e1fb
	github.com/blevesearch/bleve/v2 v2.6.0
	github.com/bohde/codel v0.2.0
	github.com/buildkite/terminal-to-html/v3 v3.16.8
	github.com/caddyserver/certmagic v0.25.3
	github.com/charmbracelet/git-lfs-transfer v0.1.1-0.20260309112543-12416315a635
	github.com/chi-middleware/proxy v1.1.1
	github.com/dimiro1/reply v0.0.0-20200315094148-d0136a4c9e21
	github.com/dlclark/regexp2 v1.12.0
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707
	github.com/dustin/go-humanize v1.0.1
	github.com/editorconfig/editorconfig-core-go/v2 v2.6.4
	github.com/emersion/go-imap v1.2.1
	github.com/emirpasic/gods v1.18.1
	github.com/ethantkoenig/rupture v1.0.1
	github.com/felixge/fgprof v0.9.5
	github.com/fsnotify/fsnotify v1.10.1
	github.com/getkin/kin-openapi v0.137.0
	github.com/gliderlabs/ssh v0.3.8
	github.com/go-chi/chi/v5 v5.2.5
	github.com/go-chi/cors v1.2.2
	github.com/go-co-op/gocron/v2 v2.21.1
	github.com/go-enry/go-enry/v2 v2.9.6
	github.com/go-git/go-billy/v5 v5.9.0
	github.com/go-git/go-git/v5 v5.19.0
	github.com/go-ldap/ldap/v3 v3.4.13
	github.com/go-redsync/redsync/v4 v4.16.0
	github.com/go-sql-driver/mysql v1.10.0
	github.com/go-webauthn/webauthn v0.17.2
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f
	github.com/gogs/go-gogs-client v0.0.0-20210131175652-1d7215cd8d85
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/go-github/v84 v84.0.0
	github.com/google/licenseclassifier/v2 v2.0.0
	github.com/google/pprof v0.0.0-20260402051712-545e8a4df936
	github.com/google/uuid v1.6.0
	github.com/gorilla/feeds v1.2.0
	github.com/gorilla/sessions v1.4.0
	github.com/hashicorp/go-version v1.9.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/huandu/xstrings v1.5.0
	github.com/jaytaylor/html2text v0.0.0-20260303211410-1a4bdc82ecec
	github.com/jhillyerd/enmime/v2 v2.3.0
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/klauspost/compress v1.18.6
	github.com/klauspost/cpuid/v2 v2.3.0
	github.com/lib/pq v1.12.3
	github.com/markbates/goth v1.82.0
	github.com/mattn/go-isatty v0.0.22
	github.com/mattn/go-sqlite3 v1.14.44
	github.com/meilisearch/meilisearch-go v0.36.2
	github.com/mholt/archives v0.1.5
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/microsoft/go-mssqldb v1.9.7
	github.com/minio/minio-go/v7 v7.1.0
	github.com/msteinert/pam/v2 v2.1.0
	github.com/niklasfasching/go-org v1.9.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/pquerna/otp v1.5.0
	github.com/prometheus/client_golang v1.23.2
	github.com/quasoft/websspi v1.1.2
	github.com/redis/go-redis/v9 v9.19.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2
	github.com/sassoftware/go-rpmutils v0.4.0
	github.com/sergi/go-diff v1.4.0
	github.com/stretchr/testify v1.11.1
	github.com/syndtr/goleveldb v1.0.0
	github.com/tstranex/u2f v1.0.0
	github.com/ulikunitz/xz v0.5.15
	github.com/urfave/cli-docs/v3 v3.1.0
	github.com/urfave/cli/v3 v3.6.1
	github.com/wneessen/go-mail v0.7.2
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/yohcop/openid-go v1.0.1
	github.com/yuin/goldmark v1.8.2
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc
	gitlab.com/gitlab-org/api/client-go v1.46.0
	go.yaml.in/yaml/v4 v4.0.0-rc.3
	golang.org/x/crypto v0.50.0
	golang.org/x/image v0.39.0
	golang.org/x/net v0.53.0
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sync v0.20.0
	golang.org/x/sys v0.44.0
	golang.org/x/text v0.36.0
	google.golang.org/grpc v1.81.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/ini.v1 v1.67.2
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.50.0
	mvdan.cc/xurls/v2 v2.6.0
	strk.kbt.io/projects/go/libravatar v0.0.0-20260301104140-add494e31dab
	xorm.io/builder v0.3.13
	xorm.io/xorm v1.3.11
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	dario.cat/mergo v1.0.2 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/DataDog/zstd v1.5.7 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/RoaringBitmap/roaring/v2 v2.16.0 // indirect
	github.com/STARRY-S/zip v0.2.3 // indirect
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.23 // indirect
	github.com/aws/smithy-go v1.25.1 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.24.4 // indirect
	github.com/blevesearch/bleve_index_api v1.3.11 // indirect
	github.com/blevesearch/geo v0.2.5 // indirect
	github.com/blevesearch/go-faiss v1.1.0 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.3 // indirect
	github.com/blevesearch/gtreap v0.1.1 // indirect
	github.com/blevesearch/mmap-go v1.2.0 // indirect
	github.com/blevesearch/scorch_segment_api/v2 v2.4.7 // indirect
	github.com/blevesearch/segment v0.9.1 // indirect
	github.com/blevesearch/snowballstem v0.9.0 // indirect
	github.com/blevesearch/upsidedown_store_api v1.0.2 // indirect
	github.com/blevesearch/vellum v1.2.0 // indirect
	github.com/blevesearch/zapx/v11 v11.4.3 // indirect
	github.com/blevesearch/zapx/v12 v12.4.3 // indirect
	github.com/blevesearch/zapx/v13 v13.4.3 // indirect
	github.com/blevesearch/zapx/v14 v14.4.3 // indirect
	github.com/blevesearch/zapx/v15 v15.4.3 // indirect
	github.com/blevesearch/zapx/v16 v16.3.4 // indirect
	github.com/blevesearch/zapx/v17 v17.1.2 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.1 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/boombuler/barcode v1.1.0 // indirect
	github.com/bradfitz/gomemcache v0.0.0-20250403215159-8d39553ac7cf // indirect
	github.com/caddyserver/zerossl v0.1.5 // indirect
	github.com/cention-sany/utf7 v0.0.0-20170124080048-26cad61bd60a // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/couchbase/go-couchbase v0.1.1 // indirect
	github.com/couchbase/gomemcached v0.3.4 // indirect
	github.com/couchbase/goutils v0.3.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/davidmz/go-pageant v1.0.2 // indirect
	github.com/emersion/go-sasl v0.0.0-20241020182733-b788ff22d5a6 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/git-lfs/pktline v0.0.0-20230103162542-ca444d533ef1 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.8-0.20250403174932-29230038a667 // indirect
	github.com/go-enry/go-oniguruma v1.2.1 // indirect
	github.com/go-fed/httpsig v1.1.1-0.20201223112313-55836744818e // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-webauthn/x v0.2.3 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/inbucket/html2text v1.0.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.6.0 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/libdns/libdns v1.1.1 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/markbates/going v1.0.3 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-runewidth v0.0.21 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/mholt/acmez/v3 v3.1.6 // indirect
	github.com/miekg/dns v1.1.72 // indirect
	github.com/mikelolasagasti/xz v1.0.1 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/minlz v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mrjones/oauth v0.0.0-20190623134757-126b35219450 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/nwaples/rardecode/v2 v2.2.2 // indirect
	github.com/oasdiff/yaml v0.0.9 // indirect
	github.com/oasdiff/yaml3 v0.0.12 // indirect
	github.com/olekukonko/cat v0.0.0-20250911104152-50322a0618f6 // indirect
	github.com/olekukonko/errors v1.2.0 // indirect
	github.com/olekukonko/ll v0.1.8 // indirect
	github.com/olekukonko/tablewriter v1.1.4 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/pjbgf/sha1cd v0.6.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rhysd/actionlint v1.7.12 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/skeema/knownhosts v1.3.2 // indirect
	github.com/sorairolake/lzip-go v0.3.8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/ssor/bom v0.0.0-20170718123548-6386211fdfcf // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/unknwon/com v1.0.1 // indirect
	github.com/woodsbury/decimal128 v1.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.etcd.io/bbolt v1.4.3 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	go4.org v0.0.0-20260112195520-a5071408f32f // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.44.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401020348-3a24fdc17823 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	modernc.org/libc v1.72.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

ignore (
	./.venv
	./node_modules
)

// When doing "go get -u ./...", Golang will try to update all dependencies
// But not all latest versions of dependencies are compatible with other packages or our codebase, so we need to pin some dependencies to specific versions
// Need to regularly maintain this list to try to update them to latest versions, especially the TODO ones

replace github.com/jaytaylor/html2text => github.com/Necoro/html2text v0.0.0-20250804200300-7bf1ce1c7347 // jaytaylor/html2text is unmaintained

replace go.yaml.in/yaml/v4 => go.yaml.in/yaml/v4 v4.0.0-rc.3 // rc.4 changes block scalar serialization, wait for stable release

replace github.com/Azure/azure-sdk-for-go/sdk/azcore => github.com/Azure/azure-sdk-for-go/sdk/azcore v1.19.0 // v1.21.0+ uses API version unsupported by Azurite in CI

replace github.com/Azure/azure-sdk-for-go/sdk/storage/azblob => github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.2 // v1.6.4+ uses API version unsupported by Azurite in CI

replace github.com/microsoft/go-mssqldb => github.com/microsoft/go-mssqldb v1.9.7 // downgraded with Azure SDK
