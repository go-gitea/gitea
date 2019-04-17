# NoDB

[中文](https://github.com/lunny/nodb/blob/master/README_CN.md)

Nodb is a fork of [ledisdb](https://github.com/siddontang/ledisdb) and shrink version. It's get rid of all C or other language codes and only keep Go's. It aims to provide a nosql database library rather than a redis like server. So if you want a redis like server, ledisdb is the best choose.

Nodb is a pure Go and high performance NoSQL database library. It supports some data structure like kv, list, hash, zset, bitmap, set.

Nodb now use [goleveldb](https://github.com/syndtr/goleveldb) as backend to store data.

## Features

+ Rich data structure: KV, List, Hash, ZSet, Bitmap, Set.
+ Stores lots of data, over the memory limit. 
+ Supports expiration and ttl.
+ Easy to embed in your own Go application.

## Install

    go get github.com/lunny/nodb

## Package Example

### Open And Select database
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

KV is the most basic nodb type like any other key-value database.
```go
err := db.Set(key, value)
value, err := db.Get(key)
```
### List

List is simply lists of values, sorted by insertion order.
You can push or pop value on the list head (left) or tail (right).
```go
err := db.LPush(key, value1)
err := db.RPush(key, value2)
value1, err := db.LPop(key)
value2, err := db.RPop(key)
```
### Hash

Hash is a map between fields and values.
```go
n, err := db.HSet(key, field1, value1)
n, err := db.HSet(key, field2, value2)
value1, err := db.HGet(key, field1)
value2, err := db.HGet(key, field2)
```
### ZSet

ZSet is a sorted collections of values.
Every member of zset is associated with score, a int64 value which used to sort, from smallest to greatest score.
Members are unique, but score may be same.
```go
n, err := db.ZAdd(key, ScorePair{score1, member1}, ScorePair{score2, member2})
ay, err := db.ZRangeByScore(key, minScore, maxScore, 0, -1)
```
## Links

+ [Ledisdb Official Website](http://ledisdb.com)
+ [GoDoc](https://godoc.org/github.com/lunny/nodb)
+ [GoWalker](https://gowalker.org/github.com/lunny/nodb)


## Thanks

Gmail: siddontang@gmail.com
