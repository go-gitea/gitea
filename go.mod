module code.gitea.io/gitea

go 1.14

require (
	cloud.google.com/go v0.45.0 // indirect
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
	github.com/RoaringBitmap/roaring v0.4.23 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/blevesearch/bleve v1.0.7
	github.com/couchbase/gomemcached v0.0.0-20191004160342-7b5da2ec40b2 // indirect
	github.com/cznic/b v0.0.0-20181122101859-a26611c4d92d // indirect
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/cznic/strutil v0.0.0-20181122101858-275e90344537 // indirect
	github.com/denisenkom/go-mssqldb v0.0.0-20200428022330-06a60b6afbbc
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/editorconfig/editorconfig-core-go/v2 v2.1.1
	github.com/emirpasic/gods v1.12.0
	github.com/ethantkoenig/rupture v0.0.0-20180203182544-0a76f03a811a
	github.com/facebookgo/ensure v0.0.0-20160127193407-b4ab57deab51 // indirect
	github.com/facebookgo/stack v0.0.0-20160209184415-751773369052 // indirect
	github.com/facebookgo/subset v0.0.0-20150612182917-8dac2c3c4870 // indirect
	github.com/gliderlabs/ssh v0.2.2
	github.com/glycerine/go-unsnap-stream v0.0.0-20190901134440-81cf024a9e0a // indirect
	github.com/go-enry/go-enry/v2 v2.5.2
	github.com/go-git/go-billy/v5 v5.0.0
	github.com/go-git/go-git/v5 v5.1.0
	github.com/go-openapi/jsonreference v0.19.3 // indirect
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/go-sql-driver/mysql v1.4.1
	github.com/go-swagger/go-swagger v0.21.0
	github.com/gobwas/glob v0.2.3
	github.com/gogs/chardet v0.0.0-20191104214054-4b6791f73a28
	github.com/gogs/cron v0.0.0-20171120032916-9f6c956d3e14
	github.com/golang/protobuf v1.4.1 // indirect
	github.com/google/go-github/v32 v32.1.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/context v1.1.1
	github.com/hashicorp/go-retryablehttp v0.6.6 // indirect
	github.com/huandu/xstrings v1.3.0
	github.com/issue9/assert v1.3.2 // indirect
	github.com/issue9/identicon v1.0.1
	github.com/jaytaylor/html2text v0.0.0-20160923191438-8fb95d837f7d
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/joho/godotenv v1.3.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20170619183022-cd60e84ee657
	github.com/keybase/go-crypto v0.0.0-20200123153347-de78d2cb44f4
	github.com/klauspost/compress v1.10.2
	github.com/lafriks/xormstore v1.3.2
	github.com/lib/pq v1.2.0
	github.com/lunny/dingtalk_webhook v0.0.0-20171025031554-e3534c89ef96
	github.com/mailru/easyjson v0.7.0 // indirect
	github.com/markbates/goth v1.61.2
	github.com/mattn/go-isatty v0.0.11
	github.com/mattn/go-oci8 v0.0.0-20190320171441-14ba190cf52d // indirect
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/mcuadros/go-version v0.0.0-20190308113854-92cdf37c5b75
	github.com/mgechev/dots v0.0.0-20190921121421-c36f7dcfbb81
	github.com/mgechev/revive v1.0.2
	github.com/microcosm-cc/bluemonday v1.0.3-0.20191119130333-0a75d7616912
	github.com/mitchellh/go-homedir v1.1.0
	github.com/msteinert/pam v0.0.0-20151204160544-02ccfbfaf0cc
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/niklasfasching/go-org v0.1.9
	github.com/oliamb/cutter v0.2.2
	github.com/olivere/elastic/v7 v7.0.9
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.2.0
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/prometheus/procfs v0.0.4 // indirect
	github.com/quasoft/websspi v1.0.0
	github.com/remyoudompheng/bigfft v0.0.0-20190321074620-2f0d2b0e0001 // indirect
	github.com/sergi/go-diff v1.1.0
	github.com/shurcooL/httpfs v0.0.0-20190527155220-6a4d4a70508b // indirect
	github.com/shurcooL/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/stretchr/testify v1.4.0
	github.com/tecbot/gorocksdb v0.0.0-20181010114359-8752a9433481 // indirect
	github.com/tinylib/msgp v1.1.2 // indirect
	github.com/tstranex/u2f v1.0.0
	github.com/unknwon/cae v1.0.0
	github.com/unknwon/com v1.0.1
	github.com/unknwon/i18n v0.0.0-20190805065654-5c6446a380b6
	github.com/unknwon/paginater v0.0.0-20151104151617-7748a72e0141
	github.com/urfave/cli v1.20.0
	github.com/xanzy/go-gitlab v0.31.0
	github.com/yohcop/openid-go v1.0.0
	github.com/yuin/goldmark v1.1.25
	github.com/yuin/goldmark-meta v0.0.0-20191126180153-f0638e958b60
	golang.org/x/crypto v0.0.0-20200604202706-70a84ac30bf9
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	golang.org/x/tools v0.0.0-20200325010219-a49f79bcc224
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20150924051756-4e86f4367175 // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/ini.v1 v1.52.0
	gopkg.in/ldap.v3 v3.0.2
	gopkg.in/testfixtures.v2 v2.5.0
	gopkg.in/yaml.v2 v2.2.8
	mvdan.cc/xurls/v2 v2.2.0
	strk.kbt.io/projects/go/libravatar v0.0.0-20191008002943-06d1c002b251
	xorm.io/builder v0.3.7
	xorm.io/xorm v1.0.1
)
