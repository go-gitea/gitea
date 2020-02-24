## log
[![GoDoc](https://godoc.org/github.com/lunny/log?status.png)](https://godoc.org/github.com/lunny/log)

[English](https://github.com/lunny/log/blob/master/README.md)

# 安装

```
go get github.com/lunny/log
```

# 特性

* 对unix增加控制台颜色支持
* 实现了保存log到数据库支持
* 实现了保存log到按日期的文件支持
* 实现了设置日期的地区

# 例子

保存到单个文件：

```Go
f, _ := os.Create("my.log")
log.Std.SetOutput(f)
```

保存到数据库：

```Go
f, _ := os.Create("my.log")
log.Std.SetOutput(io.MultiWriter(f, os.Stdout))
```

保存到按时间分隔的文件：

```Go
w := log.NewFileWriter(log.FileOptions{
    ByType:log.ByDay,
    Dir:"./logs",
})
log.Std.SetOutput(w)
```

# 关于

本 Log 是在 golang 的 log 之上的扩展

# LICENSE

 BSD License
 [http://creativecommons.org/licenses/BSD/](http://creativecommons.org/licenses/BSD/)
