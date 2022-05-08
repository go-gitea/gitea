module code.gitea.io/gitea

go 1.16

require (
	code.gitea.io/gitea-vet v0.2.1
	code.gitea.io/sdk/gitea v0.15.1
	gitea.com/go-chi/binding v0.0.0-20220309004920-114340dabecb
	gitea.com/go-chi/cache v0.0.0-20211013020926-78790b11abf1
	gitea.com/go-chi/captcha v0.0.0-20211013065431-70641c1a35d5
	gitea.com/go-chi/session v0.0.0-20211218221615-e3605d8b28b8
	gitea.com/lunny/levelqueue v0.4.1
	github.com/42wim/sshsig v0.0.0-20211121163825-841cf5bbc121
	github.com/NYTimes/gziphandler v1.1.1
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/alecthomas/chroma v0.10.0
	github.com/blevesearch/bleve/v2 v2.3.1
	github.com/caddyserver/certmagic v0.15.4
	github.com/chi-middleware/proxy v1.1.1
	github.com/denisenkom/go-mssqldb v0.12.0
	github.com/djherbis/buffer v1.2.0
	github.com/djherbis/nio/v3 v3.0.1
	github.com/duo-labs/webauthn v0.0.0-20220122034320-81aea484c951
	github.com/dustin/go-humanize v1.0.0
	github.com/editorconfig/editorconfig-core-go/v2 v2.4.2
	github.com/emirpasic/gods v1.12.0
	github.com/ethantkoenig/rupture v1.0.0
	github.com/gliderlabs/ssh v0.3.3
	github.com/go-chi/chi/v5 v5.0.7
	github.com/go-chi/cors v1.2.0
	github.com/go-enry/go-enry/v2 v2.7.1
	github.com/go-git/go-billy/v5 v5.3.1
	github.com/go-git/go-git/v5 v5.4.3-0.20210630082519-b4368b2a2ca4
	github.com/go-ldap/ldap/v3 v3.3.0
	github.com/go-redis/redis/v8 v8.11.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/go-swagger/go-swagger v0.27.0
	github.com/go-testfixtures/testfixtures/v3 v3.6.1
	github.com/gobwas/glob v0.2.3
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f
	github.com/gogs/cron v0.0.0-20171120032916-9f6c956d3e14
	github.com/gogs/go-gogs-client v0.0.0-20210131175652-1d7215cd8d85
	github.com/golang-jwt/jwt/v4 v4.3.0
	github.com/google/go-github/v39 v39.2.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/feeds v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/hashicorp/go-version v1.4.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/huandu/xstrings v1.3.2
	github.com/jaytaylor/html2text v0.0.0-20200412013138-3577fbdbcff7
	github.com/json-iterator/go v1.1.12
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/keybase/go-crypto v0.0.0-20200123153347-de78d2cb44f4
	github.com/klauspost/compress v1.13.6
	github.com/klauspost/cpuid/v2 v2.0.9
	github.com/lib/pq v1.10.2
	github.com/lunny/dingtalk_webhook v0.0.0-20171025031554-e3534c89ef96
	github.com/markbates/goth v1.69.0
	github.com/mattn/go-isatty v0.0.14
	github.com/mattn/go-sqlite3 v1.14.12
	github.com/mholt/archiver/v3 v3.5.1
	github.com/microcosm-cc/bluemonday v1.0.18
	github.com/minio/minio-go/v7 v7.0.23
	github.com/msteinert/pam v1.0.0
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/niklasfasching/go-org v1.6.2
	github.com/oliamb/cutter v0.2.2
	github.com/olivere/elastic/v7 v7.0.31
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.3.0
	github.com/prometheus/client_golang v1.12.1
	github.com/quasoft/websspi v1.1.2
	github.com/sergi/go-diff v1.2.0
	github.com/shurcooL/vfsgen v0.0.0-20200824052919-0d455de96546
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.0
	github.com/tstranex/u2f v1.0.0
	github.com/unknwon/com v1.0.1
	github.com/unknwon/i18n v0.0.0-20210321134014-0ebbf2df1c44
	github.com/unknwon/paginater v0.0.0-20200328080006-042474bd0eae
	github.com/unrolled/render v1.4.0
	github.com/urfave/cli v1.22.5
	github.com/xanzy/go-gitlab v0.58.0
	github.com/yohcop/openid-go v1.0.0
	github.com/yuin/goldmark v1.4.11
	github.com/yuin/goldmark-highlighting v0.0.0-20220208100518-594be1970594
	github.com/yuin/goldmark-meta v1.1.0
	go.jolheiser.com/hcaptcha v0.0.4
	go.jolheiser.com/pwn v0.0.3
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.21.0 // indirect
	golang.org/x/crypto v0.0.0-20220411220226-7b82a4e95df4
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f
	golang.org/x/oauth2 v0.0.0-20220223155221-ee480838109b
	golang.org/x/sys v0.0.0-20220227234510-4e6760a101f9
	golang.org/x/text v0.3.7
	golang.org/x/tools v0.1.9
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/ini.v1 v1.66.2
	gopkg.in/yaml.v2 v2.4.0
	mvdan.cc/xurls/v2 v2.2.0
	strk.kbt.io/projects/go/libravatar v0.0.0-20191008002943-06d1c002b251
	xorm.io/builder v0.3.10
	xorm.io/xorm v1.2.5
)

require (
	github.com/Azure/go-ntlmssp v0.0.0-20211209120228-48547f28849e // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20220113124808-70ae35bab23f // indirect
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/bits-and-blooms/bitset v1.2.1 // indirect
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/bradfitz/gomemcache v0.0.0-20190913173617-a41fca850d0b // indirect
	github.com/cloudflare/cfssl v1.6.1 // indirect
	github.com/couchbase/go-couchbase v0.0.0-20210224140812-5740cd35f448 // indirect
	github.com/couchbase/gomemcached v0.1.2 // indirect
	github.com/couchbase/goutils v0.0.0-20210118111533-e33d3ffb5401 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.3 // indirect
	github.com/go-openapi/analysis v0.21.2 // indirect
	github.com/go-openapi/errors v0.20.2 // indirect
	github.com/go-openapi/runtime v0.21.1 // indirect
	github.com/go-openapi/strfmt v0.21.1 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0 // indirect
	github.com/kevinburke/ssh_config v1.1.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mholt/acmez v1.0.2 // indirect
	github.com/miekg/dns v1.1.46 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mrjones/oauth v0.0.0-20190623134757-126b35219450 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.14 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/rs/xid v1.3.0 // indirect
	github.com/shurcooL/httpfs v0.0.0-20190707220628-8d4bc4ba7749 // indirect
	github.com/spf13/afero v1.8.0 // indirect
	github.com/spf13/cobra v1.3.0 // indirect
	github.com/spf13/viper v1.10.1 // indirect
	github.com/ssor/bom v0.0.0-20170718123548-6386211fdfcf // indirect
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/xanzy/ssh-agent v0.3.1 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.mongodb.org/mongo-driver v1.8.2 // indirect
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
)

replace github.com/hashicorp/go-version => github.com/6543/go-version v1.3.1

replace github.com/markbates/goth v1.68.0 => github.com/zeripath/goth v1.68.1-0.20220109111530-754359885dce

replace github.com/shurcooL/vfsgen => github.com/lunny/vfsgen v0.0.0-20220105142115-2c99e1ffdfa0

replace github.com/satori/go.uuid v1.2.0 => github.com/gofrs/uuid v4.2.0+incompatible

exclude github.com/gofrs/uuid v3.2.0+incompatible

exclude github.com/gofrs/uuid v4.0.0+incompatible

exclude github.com/goccy/go-json v0.4.11
