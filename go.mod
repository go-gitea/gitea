module code.gitea.io/gitea

go 1.14

require (
	code.gitea.io/gitea-vet v0.2.1
	code.gitea.io/sdk/gitea v0.13.1
	gitea.com/lunny/levelqueue v0.3.0
	gitea.com/macaron/binding v0.0.0-20190822013154-a5f53841ed2b
	gitea.com/macaron/cache v0.0.0-20200924044943-905232fba10b
	gitea.com/macaron/captcha v0.0.0-20200825161008-e8597820aaca
	gitea.com/macaron/cors v0.0.0-20190826180238-95aec09ea8b4
	gitea.com/macaron/csrf v0.0.0-20190822024205-3dc5a4474439
	gitea.com/macaron/gzip v0.0.0-20200827120000-efa5e8477cf5
	gitea.com/macaron/i18n v0.0.0-20200911004404-4ca3dd0cbd60
	gitea.com/macaron/inject v0.0.0-20190805023432-d4c86e31027a
	gitea.com/macaron/macaron v1.5.1-0.20201027213641-0db5d4584804
	gitea.com/macaron/session v0.0.0-20201103015045-a177a2701dee
	gitea.com/macaron/toolbox v0.0.0-20190822013122-05ff0fc766b7
	github.com/PuerkitoBio/goquery v1.5.1
	github.com/RoaringBitmap/roaring v0.5.5 // indirect
	github.com/alecthomas/chroma v0.8.1
	github.com/andybalholm/brotli v1.0.1 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/blevesearch/bleve v1.0.12
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/cznic/b v0.0.0-20181122101859-a26611c4d92d // indirect
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/cznic/strutil v0.0.0-20181122101858-275e90344537 // indirect
	github.com/denisenkom/go-mssqldb v0.9.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/editorconfig/editorconfig-core-go/v2 v2.3.8
	github.com/emirpasic/gods v1.12.0
	github.com/ethantkoenig/rupture v0.0.0-20181029165146-c3b3b810dc77
	github.com/facebookgo/ensure v0.0.0-20160127193407-b4ab57deab51 // indirect
	github.com/facebookgo/stack v0.0.0-20160209184415-751773369052 // indirect
	github.com/facebookgo/subset v0.0.0-20150612182917-8dac2c3c4870 // indirect
	github.com/gliderlabs/ssh v0.3.1
	github.com/glycerine/go-unsnap-stream v0.0.0-20190901134440-81cf024a9e0a // indirect
	github.com/go-enry/go-enry/v2 v2.5.2
	github.com/go-git/go-billy/v5 v5.0.0
	github.com/go-git/go-git/v5 v5.2.0
	github.com/go-ldap/ldap/v3 v3.2.4
	github.com/go-redis/redis/v7 v7.4.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/go-swagger/go-swagger v0.25.0
	github.com/go-testfixtures/testfixtures/v3 v3.4.1
	github.com/gobwas/glob v0.2.3
	github.com/gogs/chardet v0.0.0-20191104214054-4b6791f73a28
	github.com/gogs/cron v0.0.0-20171120032916-9f6c956d3e14
	github.com/google/go-github/v32 v32.1.0
	github.com/google/uuid v1.1.2
	github.com/gorilla/context v1.1.1
	github.com/hashicorp/go-retryablehttp v0.6.8 // indirect
	github.com/hashicorp/go-version v1.2.1
	github.com/huandu/xstrings v1.3.2
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/issue9/assert v1.3.2 // indirect
	github.com/issue9/identicon v1.0.1
	github.com/jaytaylor/html2text v0.0.0-20200412013138-3577fbdbcff7
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/keybase/go-crypto v0.0.0-20200123153347-de78d2cb44f4
	github.com/klauspost/compress v1.11.2
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/lafriks/xormstore v1.3.2
	github.com/lib/pq v1.8.1-0.20200908161135-083382b7e6fc
	github.com/lunny/dingtalk_webhook v0.0.0-20171025031554-e3534c89ef96
	github.com/markbates/goth v1.65.0
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mattn/go-isatty v0.0.12
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mattn/go-sqlite3 v1.14.4
	github.com/mgechev/dots v0.0.0-20190921121421-c36f7dcfbb81
	github.com/mgechev/revive v1.0.3-0.20200921231451-246eac737dc7
	github.com/mholt/archiver/v3 v3.5.0
	github.com/microcosm-cc/bluemonday v1.0.4
	github.com/minio/minio-go/v7 v7.0.5
	github.com/mitchellh/go-homedir v1.1.0
	github.com/msteinert/pam v0.0.0-20200810204841-913b8f8cdf8b
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/niklasfasching/go-org v1.3.2
	github.com/oliamb/cutter v0.2.2
	github.com/olivere/elastic/v7 v7.0.21
	github.com/onsi/ginkgo v1.13.0 // indirect
	github.com/pelletier/go-toml v1.8.1
	github.com/philhofer/fwd v1.1.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.2.0
	github.com/prometheus/client_golang v1.8.0
	github.com/quasoft/websspi v1.0.0
	github.com/remyoudompheng/bigfft v0.0.0-20190321074620-2f0d2b0e0001 // indirect
	github.com/sergi/go-diff v1.1.0
	github.com/shurcooL/httpfs v0.0.0-20190707220628-8d4bc4ba7749 // indirect
	github.com/shurcooL/vfsgen v0.0.0-20200824052919-0d455de96546
	github.com/spf13/viper v1.7.1 // indirect
	github.com/ssor/bom v0.0.0-20170718123548-6386211fdfcf // indirect
	github.com/stretchr/testify v1.6.1
	github.com/syndtr/goleveldb v1.0.0
	github.com/tecbot/gorocksdb v0.0.0-20181010114359-8752a9433481 // indirect
	github.com/tinylib/msgp v1.1.2 // indirect
	github.com/tstranex/u2f v1.0.0
	github.com/ulikunitz/xz v0.5.8 // indirect
	github.com/unknwon/com v1.0.1
	github.com/unknwon/i18n v0.0.0-20200823051745-09abd91c7f2c
	github.com/unknwon/paginater v0.0.0-20200328080006-042474bd0eae
	github.com/urfave/cli v1.22.5
	github.com/willf/bitset v1.1.11 // indirect
	github.com/xanzy/go-gitlab v0.39.0
	github.com/yohcop/openid-go v1.0.0
	github.com/yuin/goldmark v1.2.1
	github.com/yuin/goldmark-highlighting v0.0.0-20200307114337-60d527fdb691
	github.com/yuin/goldmark-meta v1.0.0
	go.jolheiser.com/hcaptcha v0.0.4
	go.jolheiser.com/pwn v0.0.3
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897
	golang.org/x/net v0.0.0-20201031054903-ff519b6c9102
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	golang.org/x/sys v0.0.0-20201106081118-db71ae66460a
	golang.org/x/text v0.3.4
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	golang.org/x/tools v0.0.0-20200929161345-d7fc70abf50f
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/yaml.v2 v2.3.0
	mvdan.cc/xurls/v2 v2.2.0
	strk.kbt.io/projects/go/libravatar v0.0.0-20191008002943-06d1c002b251
	xorm.io/builder v0.3.7
	xorm.io/xorm v1.0.5
)

replace github.com/hashicorp/go-version => github.com/6543/go-version v1.2.4
