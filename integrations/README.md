Integration tests can be run with make commands for the
appropriate backends, namely:

  make test-mysql
  make test-pgsql
  make test-sqlite

# Running individual tests

Example command to run GPG test with sqlite backend:

```
go test -c code.gitea.io/gitea/integrations \
  -o integrations.sqlite.test -tags 'sqlite' &&
  GITEA_ROOT="$GOPATH/src/code.gitea.io/gitea" \
  GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test \
  -test.v -test.run GPG
```

Make sure to perform a clean build before running tests:

    make clean build
