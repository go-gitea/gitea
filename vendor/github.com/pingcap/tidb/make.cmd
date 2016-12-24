@echo off
::go build option
set TiDBBuildTS=%date:~0,10% %time:~1,7%
for /f "delims=" %%i in ('git rev-parse HEAD') do (set TiDBGitHash=%%i)
set LDFLAGS="-X github.com/pingcap/tidb/util/printer.TiDBBuildTS=%TiDBBuildTS% -X github.com/pingcap/tidb/util/printer.TiDBGitHash=%TiDBGitHash%"

:: godep
go get github.com/tools/godep

@echo [Parser]
go get github.com/qiuyesuifeng/goyacc
go get github.com/qiuyesuifeng/golex
type nul >>temp.XXXXXX
goyacc -o nul -xegen "temp.XXXXXX" parser/parser.y
goyacc -o parser/parser.go -xe "temp.XXXXXX" parser/parser.y
DEL /F /A /Q temp.XXXXXX
DEL /F /A /Q y.output

golex -o parser/scanner.go parser/scanner.l

@echo [Build]
godep go build -ldflags '%LDFLAGS%'

@echo [Install] 
godep go install ./...


@echo [Test]
godep go test -cover ./...

::done
@echo [Done]
