# NoDB

[English](https://github.com/lunny/nodb/blob/master/README.md)

Nodb 是 [ledisdb](https://github.com/siddontang/ledisdb) 的克隆和缩减版本。该版本去掉了所有C和其它语言的依赖，只保留Go语言的。目标是提供一个Nosql数据库的开发库而不是提供一个像Redis那样的服务器。因此如果你想要的是一个独立服务器，你可以直接选择ledisdb。

Nodb 是一个纯Go的高性能 NoSQL 数据库。他支持 kv, list, hash, zset, bitmap, set 等数据结构。

Nodb 当前底层使用 (goleveldb)[https://github.com/syndtr/goleveldb] 来存储数据。

## 特性

+ 丰富的数据结构支持： KV, List, Hash, ZSet, Bitmap, Set。
+ 永久存储并且不受内存的限制。
+ 高性能那个。
+ 可以方便的嵌入到你的应用程序中。

## 安装

    go get github.com/lunny/nodb

## 例子

### 打开和选择数据库
```go
import(
  "github.com/lunny/nodb"
  "github.com/lunny/nodb/config"
)

cfg := new(config.Config)
cfg.DataDir = "./"
dbs, err := nodb.Open(cfg)
if err != nil {
  fmt.Printf("nodb: error opening db: %v", err)
}
db, _ := dbs.Select(0)
```
### KV

KV 是最基础的功能，和其它Nosql一样。
```go
err := db.Set(key, value)
value, err := db.Get(key)
```
### List

List 是一些值的简单列表，按照插入的顺序排列。你可以从左或右push和pop值。
```go
err := db.LPush(key, value1)
err := db.RPush(key, value2)
value1, err := db.LPop(key)
value2, err := db.RPop(key)
```
### Hash

Hash 是一个field和value对应的map。
```go
n, err := db.HSet(key, field1, value1)
n, err := db.HSet(key, field2, value2)
value1, err := db.HGet(key, field1)
value2, err := db.HGet(key, field2)
```
### ZSet

ZSet 是一个排序的值集合。zset的每个成员对应一个score，这是一个int64的值用于从小到大排序。成员不可重复，但是score可以相同。
```go
n, err := db.ZAdd(key, ScorePair{score1, member1}, ScorePair{score2, member2})
ay, err := db.ZRangeByScore(key, minScore, maxScore, 0, -1)
```

## 链接

+ [Ledisdb Official Website](http://ledisdb.com)
+ [GoDoc](https://godoc.org/github.com/lunny/nodb)
+ [GoWalker](https://gowalker.org/github.com/lunny/nodb)


## 感谢

Gmail: siddontang@gmail.com
