## log
[![GoDoc](https://godoc.org/github.com/lunny/log?status.png)](https://godoc.org/github.com/lunny/log)

[简体中文](https://github.com/lunny/log/blob/master/README_CN.md)

# Installation

```
go get github.com/lunny/log
```

# Features

* Add color support for unix console
* Implemented dbwriter to save log to database
* Implemented FileWriter to save log to file by date or time.
* Location configuration

# Example

For Single File:
```Go
f, _ := os.Create("my.log")
log.Std.SetOutput(f)
```

For Multiple Writer:
```Go
f, _ := os.Create("my.log")
log.Std.SetOutput(io.MultiWriter(f, os.Stdout))
```

For log files by date or time:
```Go
w := log.NewFileWriter(log.FileOptions{
    ByType:log.ByDay,
    Dir:"./logs",
})
log.Std.SetOutput(w)
```

# About

This repo is an extension of Golang log.

# LICENSE

 BSD License
 [http://creativecommons.org/licenses/BSD/](http://creativecommons.org/licenses/BSD/)
