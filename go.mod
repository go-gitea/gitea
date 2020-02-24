module code.gitea.io/gitea

go 1.13

require (
	cloud.google.com/go v0.45.0 // indirect
	gitea.com/lunny/levelqueue v0.1.0
	gitea.com/macaron/binding v0.0.0-20190822013154-a5f53841ed2b
	gitea.com/macaron/cache v0.0.0-20190822004001-a6e7fee4ee76
	gitea.com/macaron/captcha v0.0.0-20190822015246-daa973478bae
	gitea.com/macaron/cors v0.0.0-20190821152825-7dcef4a17175
	gitea.com/macaron/csrf v0.0.0-20190822024205-3dc5a4474439
	gitea.com/macaron/gzip v0.0.0-20191118033930-0c4c5566a0e5
	gitea.com/macaron/i18n v0.0.0-20190822004228-474e714e2223
	gitea.com/macaron/inject v0.0.0-20190805023432-d4c86e31027a
	gitea.com/macaron/macaron v1.4.0
	gitea.com/macaron/session v0.0.0-20190821211443-122c47c5f705
	gitea.com/macaron/toolbox v0.0.0-20190822013122-05ff0fc766b7
	github.com/PuerkitoBio/goquery v1.5.0
	github.com/RoaringBitmap/roaring v0.4.21 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/blevesearch/bleve v0.8.1
	github.com/blevesearch/blevex v0.0.0-20180227211930-4b158bb555a3 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.2 // indirect
	github.com/blevesearch/segment v0.0.0-20160915185041-762005e7a34f // indirect
	github.com/boombuler/barcode v0.0.0-20161226211916-fe0f26ff6d26 // indirect
	github.com/couchbase/gomemcached v0.0.0-20191004160342-7b5da2ec40b2 // indirect
	github.com/couchbase/goutils v0.0.0-20191018232750-b49639060d85 // indirect
	github.com/couchbase/vellum v0.0.0-20190829182332-ef2e028c01fd // indirect
	github.com/cznic/b v0.0.0-20181122101859-a26611c4d92d // indirect
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/cznic/strutil v0.0.0-20181122101858-275e90344537 // indirect
	github.com/denisenkom/go-mssqldb v0.0.0-20191128021309-1d7a30a10f73
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/editorconfig/editorconfig-core-go/v2 v2.1.1
	github.com/emirpasic/gods v1.12.0
	github.com/etcd-io/bbolt v1.3.3 // indirect
	github.com/ethantkoenig/rupture v0.0.0-20180203182544-0a76f03a811a
	github.com/facebookgo/ensure v0.0.0-20160127193407-b4ab57deab51 // indirect
	github.com/facebookgo/stack v0.0.0-20160209184415-751773369052 // indirect
	github.com/facebookgo/subset v0.0.0-20150612182917-8dac2c3c4870 // indirect
	github.com/gliderlabs/ssh v0.2.2
	github.com/glycerine/go-unsnap-stream v0.0.0-20190901134440-81cf024a9e0a // indirect
	github.com/go-openapi/jsonreference v0.19.3 // indirect
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/go-sql-driver/mysql v1.4.1
	github.com/go-swagger/go-swagger v0.21.0
	github.com/gobwas/glob v0.2.3
	github.com/gogs/chardet v0.0.0-20191104214054-4b6791f73a28
	github.com/gogs/cron v0.0.0-20171120032916-9f6c956d3e14
	github.com/google/go-github/v24 v24.0.1
	github.com/gorilla/context v1.1.1
	github.com/issue9/assert v1.3.2 // indirect
	github.com/issue9/identicon v0.0.0-20160320065130-d36b54562f4c
	github.com/jaytaylor/html2text v0.0.0-20160923191438-8fb95d837f7d
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/joho/godotenv v1.3.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20170619183022-cd60e84ee657
	github.com/keybase/go-crypto v0.0.0-20170605145657-00ac4db533f6
	github.com/klauspost/compress v1.9.2
	github.com/lafriks/xormstore v1.3.2
	github.com/lib/pq v1.2.0
	github.com/lunny/dingtalk_webhook v0.0.0-20171025031554-e3534c89ef96
	github.com/mailru/easyjson v0.7.0 // indirect
	github.com/markbates/goth v1.61.2
	github.com/mattn/go-isatty v0.0.7
	github.com/mattn/go-oci8 v0.0.0-20190320171441-14ba190cf52d // indirect
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/mcuadros/go-version v0.0.0-20190308113854-92cdf37c5b75
	github.com/microcosm-cc/bluemonday v0.0.0-20161012083705-f77f16ffc87a
	github.com/msteinert/pam v0.0.0-20151204160544-02ccfbfaf0cc
	github.com/nfnt/resize v0.0.0-20160724205520-891127d8d1b5
	github.com/niklasfasching/go-org v0.1.8
	github.com/oliamb/cutter v0.2.2
	github.com/pkg/errors v0.8.1
	github.com/pquerna/otp v0.0.0-20160912161815-54653902c20e
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/prometheus/procfs v0.0.4 // indirect
	github.com/quasoft/websspi v1.0.0
	github.com/remyoudompheng/bigfft v0.0.0-20190321074620-2f0d2b0e0001 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sergi/go-diff v1.0.0
	github.com/shurcooL/httpfs v0.0.0-20190527155220-6a4d4a70508b // indirect
	github.com/shurcooL/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/steveyen/gtreap v0.0.0-20150807155958-0abe01ef9be2 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/tecbot/gorocksdb v0.0.0-20181010114359-8752a9433481 // indirect
	github.com/tstranex/u2f v1.0.0
	github.com/unknwon/cae v0.0.0-20190822084630-55a0b64484a1
	github.com/unknwon/com v1.0.1
	github.com/unknwon/i18n v0.0.0-20190805065654-5c6446a380b6
	github.com/unknwon/paginater v0.0.0-20151104151617-7748a72e0141
	github.com/urfave/cli v1.20.0
	github.com/yohcop/openid-go v0.0.0-20160914080427-2c050d2dae53
	github.com/yuin/goldmark v1.1.19
	go.etcd.io/bbolt v1.3.3 // indirect
	golang.org/x/crypto v0.0.0-20200219234226-1ad67e1f0ef4
	golang.org/x/net v0.0.0-20191101175033-0deb6923b6d9
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20200219091948-cb0a6d8edb6c
	golang.org/x/text v0.3.2
	golang.org/x/tools v0.0.0-20191213221258-04c2e8eff935 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20150924051756-4e86f4367175 // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/ini.v1 v1.51.1
	gopkg.in/ldap.v3 v3.0.2
	gopkg.in/src-d/go-billy.v4 v4.3.2
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/testfixtures.v2 v2.5.0
	mvdan.cc/xurls/v2 v2.1.0
	strk.kbt.io/projects/go/libravatar v0.0.0-20191008002943-06d1c002b251
	xorm.io/builder v0.3.6
	xorm.io/core v0.7.2
	xorm.io/xorm v0.8.1
)
