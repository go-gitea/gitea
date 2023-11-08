module code.gitea.io/gitea

go 1.21

require (
	code.gitea.io/actions-proto-go v0.3.1
	code.gitea.io/gitea-vet v0.2.3
	code.gitea.io/sdk/gitea v0.16.0
	codeberg.org/gusted/mcaptcha v0.0.0-20220723083913-4f3072e1d570
	gitea.com/go-chi/binding v0.0.0-20230415142243-04b515c6d669
	gitea.com/go-chi/cache v0.2.0
	gitea.com/go-chi/captcha v0.0.0-20230415143339-2c0754df4384
	gitea.com/go-chi/session v0.0.0-20230613035928-39541325faa3
	gitea.com/lunny/dingtalk_webhook v0.0.0-20171025031554-e3534c89ef96
	gitea.com/lunny/levelqueue v0.4.2-0.20230414023320-3c0159fe0fe4
	github.com/42wim/sshsig v0.0.0-20211121163825-841cf5bbc121
	github.com/Azure/go-ntlmssp v0.0.0-20221128193559-754e69321358
	github.com/NYTimes/gziphandler v1.1.1
	github.com/PuerkitoBio/goquery v1.8.1
	github.com/alecthomas/chroma/v2 v2.10.0
	github.com/blakesmith/ar v0.0.0-20190502131153-809d4375e1fb
	github.com/blevesearch/bleve/v2 v2.3.10
	github.com/bufbuild/connect-go v1.10.0
	github.com/buildkite/terminal-to-html/v3 v3.9.1
	github.com/caddyserver/certmagic v0.19.2
	github.com/chi-middleware/proxy v1.1.1
	github.com/denisenkom/go-mssqldb v0.12.3
	github.com/dimiro1/reply v0.0.0-20200315094148-d0136a4c9e21
	github.com/djherbis/buffer v1.2.0
	github.com/djherbis/nio/v3 v3.0.1
	github.com/dsnet/compress v0.0.2-0.20210315054119-f66993602bf5
	github.com/dustin/go-humanize v1.0.1
	github.com/editorconfig/editorconfig-core-go/v2 v2.6.0
	github.com/emersion/go-imap v1.2.1
	github.com/emirpasic/gods v1.18.1
	github.com/ethantkoenig/rupture v1.0.1
	github.com/felixge/fgprof v0.9.3
	github.com/fsnotify/fsnotify v1.6.0
	github.com/gliderlabs/ssh v0.3.6-0.20230927171611-ece6c7995e46
	github.com/go-ap/activitypub v0.0.0-20231003111253-1fba3772399b
	github.com/go-ap/jsonld v0.0.0-20221030091449-f2a191312c73
	github.com/go-chi/chi/v5 v5.0.10
	github.com/go-chi/cors v1.2.1
	github.com/go-co-op/gocron v1.31.1
	github.com/go-enry/go-enry/v2 v2.8.6
	github.com/go-fed/httpsig v1.1.1-0.20201223112313-55836744818e
	github.com/go-git/go-billy/v5 v5.5.0
	github.com/go-git/go-git/v5 v5.9.0
	github.com/go-ldap/ldap/v3 v3.4.6
	github.com/go-sql-driver/mysql v1.7.1
	github.com/go-swagger/go-swagger v0.30.5
	github.com/go-testfixtures/testfixtures/v3 v3.9.0
	github.com/go-webauthn/webauthn v0.8.6
	github.com/gobwas/glob v0.2.3
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f
	github.com/gogs/go-gogs-client v0.0.0-20210131175652-1d7215cd8d85
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/google/go-github/v53 v53.2.0
	github.com/google/pprof v0.0.0-20230926050212-f7f687d19a98
	github.com/google/uuid v1.3.1
	github.com/gorilla/feeds v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/hashicorp/go-version v1.6.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/huandu/xstrings v1.4.0
	github.com/jaytaylor/html2text v0.0.0-20230321000545-74c2419ad056
	github.com/jhillyerd/enmime v1.0.1
	github.com/json-iterator/go v1.1.12
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/keybase/go-crypto v0.0.0-20200123153347-de78d2cb44f4
	github.com/klauspost/compress v1.17.0
	github.com/klauspost/cpuid/v2 v2.2.5
	github.com/lib/pq v1.10.9
	github.com/markbates/goth v1.78.0
	github.com/mattn/go-isatty v0.0.19
	github.com/mattn/go-sqlite3 v1.14.17
	github.com/meilisearch/meilisearch-go v0.25.1
	github.com/mholt/archiver/v3 v3.5.1
	github.com/microcosm-cc/bluemonday v1.0.26
	github.com/minio/minio-go/v7 v7.0.63
	github.com/minio/sha256-simd v1.0.1
	github.com/msteinert/pam v1.2.0
	github.com/nektos/act v0.2.52
	github.com/niklasfasching/go-org v1.7.0
	github.com/olivere/elastic/v7 v7.0.32
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc5
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.4.0
	github.com/prometheus/client_golang v1.17.0
	github.com/quasoft/websspi v1.1.2
	github.com/redis/go-redis/v9 v9.2.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/sassoftware/go-rpmutils v0.2.0
	github.com/sergi/go-diff v1.3.1
	github.com/shurcooL/vfsgen v0.0.0-20230704071429-0000e147ea92
	github.com/stretchr/testify v1.8.4
	github.com/syndtr/goleveldb v1.0.0
	github.com/tstranex/u2f v1.0.0
	github.com/ulikunitz/xz v0.5.11
	github.com/urfave/cli/v2 v2.25.7
	github.com/xanzy/go-gitlab v0.93.1
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/yohcop/openid-go v1.0.1
	github.com/yuin/goldmark v1.5.6
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc
	github.com/yuin/goldmark-meta v1.1.0
	golang.org/x/crypto v0.14.0
	golang.org/x/image v0.13.0
	golang.org/x/net v0.17.0
	golang.org/x/oauth2 v0.13.0
	golang.org/x/sys v0.13.0
	golang.org/x/text v0.13.0
	golang.org/x/tools v0.14.0
	google.golang.org/grpc v1.58.3
	google.golang.org/protobuf v1.31.0
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/ini.v1 v1.67.0
	gopkg.in/yaml.v3 v3.0.1
	mvdan.cc/xurls/v2 v2.5.0
	strk.kbt.io/projects/go/libravatar v0.0.0-20191008002943-06d1c002b251
	xorm.io/builder v0.3.13
	xorm.io/xorm v1.3.4
)

require (
	cloud.google.com/go/compute v1.23.1 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	dario.cat/mergo v1.0.0 // indirect
	git.sr.ht/~mariusor/go-xsd-duration v0.0.0-20220703122237-02e73435a078 // indirect
	github.com/ClickHouse/ch-go v0.58.2 // indirect
	github.com/ClickHouse/clickhouse-go/v2 v2.14.3 // indirect
	github.com/DataDog/zstd v1.5.5 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/Masterminds/sprig/v3 v3.2.3 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20230923063757-afb1ddc0824c // indirect
	github.com/RoaringBitmap/roaring v1.6.0 // indirect
	github.com/acomagu/bufpipe v1.0.4 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.9.0 // indirect
	github.com/blevesearch/bleve_index_api v1.0.6 // indirect
	github.com/blevesearch/geo v0.1.18 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.3 // indirect
	github.com/blevesearch/gtreap v0.1.1 // indirect
	github.com/blevesearch/mmap-go v1.0.4 // indirect
	github.com/blevesearch/scorch_segment_api/v2 v2.1.6 // indirect
	github.com/blevesearch/segment v0.9.1 // indirect
	github.com/blevesearch/snowballstem v0.9.0 // indirect
	github.com/blevesearch/upsidedown_store_api v1.0.2 // indirect
	github.com/blevesearch/vellum v1.0.10 // indirect
	github.com/blevesearch/zapx/v11 v11.3.10 // indirect
	github.com/blevesearch/zapx/v12 v12.3.10 // indirect
	github.com/blevesearch/zapx/v13 v13.3.10 // indirect
	github.com/blevesearch/zapx/v14 v14.3.10 // indirect
	github.com/blevesearch/zapx/v15 v15.3.13 // indirect
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/bradfitz/gomemcache v0.0.0-20230905024940-24af94b03874 // indirect
	github.com/cention-sany/utf7 v0.0.0-20170124080048-26cad61bd60a // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudflare/circl v1.3.3 // indirect
	github.com/couchbase/go-couchbase v0.1.1 // indirect
	github.com/couchbase/gomemcached v0.2.1 // indirect
	github.com/couchbase/goutils v0.1.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.3 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/davidmz/go-pageant v1.0.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/emersion/go-sasl v0.0.0-20220912192320-0145f2c60ead // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fxamacker/cbor/v2 v2.5.0 // indirect
	github.com/go-ap/errors v0.0.0-20231003111023-183eef4b31b7 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.5 // indirect
	github.com/go-enry/go-oniguruma v1.2.1 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.6.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.20.4 // indirect
	github.com/go-openapi/inflect v0.19.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.26.0 // indirect
	github.com/go-openapi/spec v0.20.9 // indirect
	github.com/go-openapi/strfmt v0.21.7 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/go-openapi/validate v0.22.1 // indirect
	github.com/go-webauthn/x v0.1.4 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/geo v0.0.0-20230421003525-6adc56603217 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/go-tpm v0.9.0 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jessevdk/go-flags v1.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/markbates/going v1.0.3 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mholt/acmez v1.2.0 // indirect
	github.com/miekg/dns v1.1.56 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mrjones/oauth v0.0.0-20190623134757-126b35219450 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/paulmach/orb v0.10.0 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/rhysd/actionlint v1.6.26 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.3.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/shurcooL/httpfs v0.0.0-20230704072500-f1e31cf0ba5c // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/skeema/knownhosts v1.2.1 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.10.0 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.17.0 // indirect
	github.com/ssor/bom v0.0.0-20170718123548-6386211fdfcf // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/toqueteos/webbrowser v1.2.0 // indirect
	github.com/unknwon/com v1.0.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.50.0 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	go.etcd.io/bbolt v1.3.7 // indirect
	go.mongodb.org/mongo-driver v1.12.1 // indirect
	go.opentelemetry.io/otel v1.19.0 // indirect
	go.opentelemetry.io/otel/trace v1.19.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d // indirect
	golang.org/x/mod v0.13.0 // indirect
	golang.org/x/sync v0.4.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231012201019-e917dd12ba7a // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/hashicorp/go-version => github.com/6543/go-version v1.3.1

replace github.com/shurcooL/vfsgen => github.com/lunny/vfsgen v0.0.0-20220105142115-2c99e1ffdfa0

replace github.com/nektos/act => gitea.com/gitea/act v0.2.51

exclude github.com/gofrs/uuid v3.2.0+incompatible

exclude github.com/gofrs/uuid v4.0.0+incompatible

exclude github.com/goccy/go-json v0.4.11

exclude github.com/satori/go.uuid v1.2.0
