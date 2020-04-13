# go-shlex

go-shlex is a library to make a lexical analyzer like Unix shell for
Go.

## Install

    go get -u "github.com/anmitsu/go-shlex"

## Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/anmitsu/go-shlex"
)

func main() {
    cmd := `cp -Rdp "file name" 'file name2' dir\ name`
    words, err := shlex.Split(cmd, true)
    if err != nil {
        log.Fatal(err)
    }

    for _, w := range words {
        fmt.Println(w)
    }
}
```

## Documentation

http://godoc.org/github.com/anmitsu/go-shlex

