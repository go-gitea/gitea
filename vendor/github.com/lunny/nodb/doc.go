// package nodb is a high performance embedded NoSQL.
//
// nodb supports various data structure like kv, list, hash and zset like redis.
//
// Other features include binlog replication, data with a limited time-to-live.
//
// Usage
//
// First create a nodb instance before use:
//
//  l := nodb.Open(cfg)
//
// cfg is a Config instance which contains configuration for nodb use,
// like DataDir (root directory for nodb working to store data).
//
// After you create a nodb instance, you can select a DB to store you data:
//
//  db, _ := l.Select(0)
//
// DB must be selected by a index, nodb supports only 16 databases, so the index range is [0-15].
//
// KV
//
// KV is the most basic nodb type like any other key-value database.
//
//  err := db.Set(key, value)
//  value, err := db.Get(key)
//
// List
//
// List is simply lists of values, sorted by insertion order.
// You can push or pop value on the list head (left) or tail (right).
//
//  err := db.LPush(key, value1)
//  err := db.RPush(key, value2)
//  value1, err := db.LPop(key)
//  value2, err := db.RPop(key)
//
// Hash
//
// Hash is a map between fields and values.
//
//  n, err := db.HSet(key, field1, value1)
//  n, err := db.HSet(key, field2, value2)
//  value1, err := db.HGet(key, field1)
//  value2, err := db.HGet(key, field2)
//
// ZSet
//
// ZSet is a sorted collections of values.
// Every member of zset is associated with score, a int64 value which used to sort, from smallest to greatest score.
// Members are unique, but score may be same.
//
//  n, err := db.ZAdd(key, ScorePair{score1, member1}, ScorePair{score2, member2})
//  ay, err := db.ZRangeByScore(key, minScore, maxScore, 0, -1)
//
// Binlog
//
// nodb supports binlog, so you can sync binlog to another server for replication. If you want to open binlog support, set UseBinLog to true in config.
//
package nodb
