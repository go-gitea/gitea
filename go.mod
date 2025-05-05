module code.gitea.io/gitea

go 1.23.0

toolchain go1.24.1

require (
	gitea.com/jolheiser/gitea-vet v0.1.0
	gitea.com/lunny/levelqueue v0.3.0
	gitea.com/macaron/binding v0.0.0-20190822013154-a5f53841ed2b
	gitea.com/macaron/cache v0.0.0-20190822004001-a6e7fee4ee76
	gitea.com/macaron/captcha v0.0.0-20190822015246-daa973478bae
	gitea.com/macaron/cors v0.0.0-20190826180238-95aec09ea8b4
	gitea.com/macaron/csrf v0.0.0-20190822024205-3dc5a4474439
	gitea.com/macaron/gzip v0.0.0-20191118041502-506895b47aae
	gitea.com/macaron/i18n v0.0.0-20190822004228-474e714e2223
	gitea.com/macaron/inject v0.0.0-20190805023432-d4c86e31027a
	gitea.com/macaron/macaron v1.4.0
	gitea.com/macaron/session v0.0.0-20200902202411-e3a87877db6e
	gitea.com/macaron/toolbox v0.0.0-20190822013122-05ff0fc766b7
	github.com/BurntSushi/toml v0.3.1
	github.com/PuerkitoBio/goquery v1.5.0
	github.com/blevesearch/bleve v1.0.11
	github.com/denisenkom/go-mssqldb v0.0.0-20200428022330-06a60b6afbbc
	github.com/dustin/go-humanize v1.0.0
	github.com/editorconfig/editorconfig-core-go/v2 v2.1.1
	github.com/emirpasic/gods v1.18.1
	github.com/ethantkoenig/rupture v0.0.0-20180203182544-0a76f03a811a
	github.com/gliderlabs/ssh v0.3.8
	github.com/go-enry/go-enry/v2 v2.5.2
	github.com/go-git/go-billy/v5 v5.6.0
	github.com/go-git/go-git/v5 v5.13.0
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/go-sql-driver/mysql v1.4.1
	github.com/go-swagger/go-swagger v0.21.0
	github.com/gobwas/glob v0.2.3
	github.com/gogs/chardet v0.0.0-20191104214054-4b6791f73a28
	github.com/gogs/cron v0.0.0-20171120032916-9f6c956d3e14
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/google/go-github/v32 v32.1.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/context v1.1.1
	github.com/huandu/xstrings v1.3.0
	github.com/issue9/identicon v1.0.1
	github.com/jaytaylor/html2text v0.0.0-20160923191438-8fb95d837f7d
	github.com/kballard/go-shellquote v0.0.0-20170619183022-cd60e84ee657
	github.com/keybase/go-crypto v0.0.0-20200123153347-de78d2cb44f4
	github.com/klauspost/compress v1.10.2
	github.com/lafriks/xormstore v1.3.2
	github.com/lib/pq v1.2.0
	github.com/lunny/dingtalk_webhook v0.0.0-20171025031554-e3534c89ef96
	github.com/markbates/goth v1.61.2
	github.com/mattn/go-isatty v0.0.20
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/mcuadros/go-version v0.0.0-20190308113854-92cdf37c5b75
	github.com/mgechev/dots v0.0.0-20190921121421-c36f7dcfbb81
	github.com/mgechev/revive v1.0.2
	github.com/microcosm-cc/bluemonday v1.0.16
	github.com/mitchellh/go-homedir v1.1.0
	github.com/msteinert/pam v0.0.0-20151204160544-02ccfbfaf0cc
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/niklasfasching/go-org v0.1.9
	github.com/oliamb/cutter v0.2.2
	github.com/olivere/elastic/v7 v7.0.9
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.2.0
	github.com/prometheus/client_golang v1.11.1
	github.com/quasoft/websspi v1.0.0
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3
	github.com/shurcooL/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/stretchr/testify v1.10.0
	github.com/tstranex/u2f v1.0.0
	github.com/unknwon/cae v1.0.1
	github.com/unknwon/com v1.0.1
	github.com/unknwon/i18n v0.0.0-20190805065654-5c6446a380b6
	github.com/unknwon/paginater v0.0.0-20151104151617-7748a72e0141
	github.com/urfave/cli v1.20.0
	github.com/xanzy/go-gitlab v0.31.0
	github.com/yohcop/openid-go v1.0.0
	github.com/yuin/goldmark v1.4.13
	github.com/yuin/goldmark-meta v0.0.0-20191126180153-f0638e958b60
	golang.org/x/crypto v0.36.0
	golang.org/x/net v0.38.0
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.31.0
	golang.org/x/text v0.23.0
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/ini.v1 v1.52.0
	gopkg.in/ldap.v3 v3.0.2
	gopkg.in/testfixtures.v2 v2.5.0
	gopkg.in/yaml.v2 v2.4.0
	mvdan.cc/xurls/v2 v2.2.0
	strk.kbt.io/projects/go/libravatar v0.0.0-20191008002943-06d1c002b251
	xorm.io/builder v0.3.7
	xorm.io/xorm v1.0.1
)

require (
	cloud.google.com/go v0.45.0 // indirect
	dario.cat/mergo v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/ProtonMail/go-crypto v1.1.3 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/RoaringBitmap/roaring v0.4.23 // indirect
	github.com/andybalholm/cascadia v1.0.0 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.3 // indirect
	github.com/blevesearch/mmap-go v1.0.2 // indirect
	github.com/blevesearch/segment v0.9.0 // indirect
	github.com/blevesearch/snowballstem v0.9.0 // indirect
	github.com/blevesearch/zap/v11 v11.0.11 // indirect
	github.com/blevesearch/zap/v12 v12.0.11 // indirect
	github.com/blevesearch/zap/v13 v13.0.3 // indirect
	github.com/blevesearch/zap/v14 v14.0.2 // indirect
	github.com/blevesearch/zap/v15 v15.0.0 // indirect
	github.com/boombuler/barcode v1.0.1-0.20190219062509-6c824513bacc // indirect
	github.com/bradfitz/gomemcache v0.0.0-20190329173943-551aad21a668 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/couchbase/gomemcached v0.0.0-20191004160342-7b5da2ec40b2 // indirect
	github.com/couchbase/goutils v0.0.0-20191018232750-b49639060d85 // indirect
	github.com/couchbase/vellum v1.0.2 // indirect
	github.com/couchbaselabs/go-couchbase v0.0.0-20190708161019-23e7ca2ce2b7 // indirect
	github.com/cyphar/filepath-securejoin v0.2.5 // indirect
	github.com/cznic/b v0.0.0-20181122101859-a26611c4d92d // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/fsnotify/fsnotify v1.4.7 // indirect
	github.com/glycerine/go-unsnap-stream v0.0.0-20190901134440-81cf024a9e0a // indirect
	github.com/go-enry/go-oniguruma v1.2.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-openapi/analysis v0.19.5 // indirect
	github.com/go-openapi/errors v0.19.2 // indirect
	github.com/go-openapi/inflect v0.19.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.3 // indirect
	github.com/go-openapi/jsonreference v0.19.3 // indirect
	github.com/go-openapi/loads v0.19.3 // indirect
	github.com/go-openapi/runtime v0.19.5 // indirect
	github.com/go-openapi/spec v0.19.3 // indirect
	github.com/go-openapi/strfmt v0.19.3 // indirect
	github.com/go-openapi/swag v0.19.5 // indirect
	github.com/go-openapi/validate v0.19.3 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.0 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/gorilla/handlers v1.4.2 // indirect
	github.com/gorilla/mux v1.6.2 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/gorilla/sessions v1.2.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/issue9/assert v1.3.2 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jessevdk/go-flags v1.4.0 // indirect
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/joho/godotenv v1.3.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lunny/log v0.0.0-20160921050905-7887c61bf0de // indirect
	github.com/lunny/nodb v0.0.0-20160621015157-fc1ef06ad4af // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mailru/easyjson v0.7.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-oci8 v0.0.0-20190320171441-14ba190cf52d // indirect
	github.com/mattn/go-runewidth v0.0.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.1.2 // indirect
	github.com/mrjones/oauth v0.0.0-20180629183705-f4e24b6d100c // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/shurcooL/httpfs v0.0.0-20190527155220-6a4d4a70508b // indirect
	github.com/siddontang/go-snappy v0.0.0-20140704025258-d8f7bb82a96d // indirect
	github.com/skeema/knownhosts v1.3.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/spf13/viper v1.4.0 // indirect
	github.com/steveyen/gtreap v0.1.0 // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/tecbot/gorocksdb v0.0.0-20181010114359-8752a9433481 // indirect
	github.com/tinylib/msgp v1.1.2 // indirect
	github.com/toqueteos/webbrowser v1.2.0 // indirect
	github.com/willf/bitset v1.1.10 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.etcd.io/bbolt v1.3.5 // indirect
	go.mongodb.org/mongo-driver v1.1.1 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20150924051756-4e86f4367175 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/blevesearch/zap/v15 => github.com/blevesearch/zap/v14 v14.0.0
