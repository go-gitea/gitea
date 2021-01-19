package redis

import (
	"errors"
	"io"
	"time"

	"github.com/go-redis/redis/v7/internal"
)

func usePrecise(dur time.Duration) bool {
	return dur < time.Second || dur%time.Second != 0
}

func formatMs(dur time.Duration) int64 {
	if dur > 0 && dur < time.Millisecond {
		internal.Logger.Printf(
			"specified duration is %s, but minimal supported value is %s",
			dur, time.Millisecond,
		)
	}
	return int64(dur / time.Millisecond)
}

func formatSec(dur time.Duration) int64 {
	if dur > 0 && dur < time.Second {
		internal.Logger.Printf(
			"specified duration is %s, but minimal supported value is %s",
			dur, time.Second,
		)
	}
	return int64(dur / time.Second)
}

func appendArgs(dst, src []interface{}) []interface{} {
	if len(src) == 1 {
		switch v := src[0].(type) {
		case []string:
			for _, s := range v {
				dst = append(dst, s)
			}
			return dst
		case map[string]interface{}:
			for k, v := range v {
				dst = append(dst, k, v)
			}
			return dst
		}
	}

	dst = append(dst, src...)
	return dst
}

type Cmdable interface {
	Pipeline() Pipeliner
	Pipelined(fn func(Pipeliner) error) ([]Cmder, error)

	TxPipelined(fn func(Pipeliner) error) ([]Cmder, error)
	TxPipeline() Pipeliner

	Command() *CommandsInfoCmd
	ClientGetName() *StringCmd
	Echo(message interface{}) *StringCmd
	Ping() *StatusCmd
	Quit() *StatusCmd
	Del(keys ...string) *IntCmd
	Unlink(keys ...string) *IntCmd
	Dump(key string) *StringCmd
	Exists(keys ...string) *IntCmd
	Expire(key string, expiration time.Duration) *BoolCmd
	ExpireAt(key string, tm time.Time) *BoolCmd
	Keys(pattern string) *StringSliceCmd
	Migrate(host, port, key string, db int, timeout time.Duration) *StatusCmd
	Move(key string, db int) *BoolCmd
	ObjectRefCount(key string) *IntCmd
	ObjectEncoding(key string) *StringCmd
	ObjectIdleTime(key string) *DurationCmd
	Persist(key string) *BoolCmd
	PExpire(key string, expiration time.Duration) *BoolCmd
	PExpireAt(key string, tm time.Time) *BoolCmd
	PTTL(key string) *DurationCmd
	RandomKey() *StringCmd
	Rename(key, newkey string) *StatusCmd
	RenameNX(key, newkey string) *BoolCmd
	Restore(key string, ttl time.Duration, value string) *StatusCmd
	RestoreReplace(key string, ttl time.Duration, value string) *StatusCmd
	Sort(key string, sort *Sort) *StringSliceCmd
	SortStore(key, store string, sort *Sort) *IntCmd
	SortInterfaces(key string, sort *Sort) *SliceCmd
	Touch(keys ...string) *IntCmd
	TTL(key string) *DurationCmd
	Type(key string) *StatusCmd
	Scan(cursor uint64, match string, count int64) *ScanCmd
	SScan(key string, cursor uint64, match string, count int64) *ScanCmd
	HScan(key string, cursor uint64, match string, count int64) *ScanCmd
	ZScan(key string, cursor uint64, match string, count int64) *ScanCmd
	Append(key, value string) *IntCmd
	BitCount(key string, bitCount *BitCount) *IntCmd
	BitOpAnd(destKey string, keys ...string) *IntCmd
	BitOpOr(destKey string, keys ...string) *IntCmd
	BitOpXor(destKey string, keys ...string) *IntCmd
	BitOpNot(destKey string, key string) *IntCmd
	BitPos(key string, bit int64, pos ...int64) *IntCmd
	BitField(key string, args ...interface{}) *IntSliceCmd
	Decr(key string) *IntCmd
	DecrBy(key string, decrement int64) *IntCmd
	Get(key string) *StringCmd
	GetBit(key string, offset int64) *IntCmd
	GetRange(key string, start, end int64) *StringCmd
	GetSet(key string, value interface{}) *StringCmd
	Incr(key string) *IntCmd
	IncrBy(key string, value int64) *IntCmd
	IncrByFloat(key string, value float64) *FloatCmd
	MGet(keys ...string) *SliceCmd
	MSet(values ...interface{}) *StatusCmd
	MSetNX(values ...interface{}) *BoolCmd
	Set(key string, value interface{}, expiration time.Duration) *StatusCmd
	SetBit(key string, offset int64, value int) *IntCmd
	SetNX(key string, value interface{}, expiration time.Duration) *BoolCmd
	SetXX(key string, value interface{}, expiration time.Duration) *BoolCmd
	SetRange(key string, offset int64, value string) *IntCmd
	StrLen(key string) *IntCmd
	HDel(key string, fields ...string) *IntCmd
	HExists(key, field string) *BoolCmd
	HGet(key, field string) *StringCmd
	HGetAll(key string) *StringStringMapCmd
	HIncrBy(key, field string, incr int64) *IntCmd
	HIncrByFloat(key, field string, incr float64) *FloatCmd
	HKeys(key string) *StringSliceCmd
	HLen(key string) *IntCmd
	HMGet(key string, fields ...string) *SliceCmd
	HSet(key string, values ...interface{}) *IntCmd
	HMSet(key string, values ...interface{}) *BoolCmd
	HSetNX(key, field string, value interface{}) *BoolCmd
	HVals(key string) *StringSliceCmd
	BLPop(timeout time.Duration, keys ...string) *StringSliceCmd
	BRPop(timeout time.Duration, keys ...string) *StringSliceCmd
	BRPopLPush(source, destination string, timeout time.Duration) *StringCmd
	LIndex(key string, index int64) *StringCmd
	LInsert(key, op string, pivot, value interface{}) *IntCmd
	LInsertBefore(key string, pivot, value interface{}) *IntCmd
	LInsertAfter(key string, pivot, value interface{}) *IntCmd
	LLen(key string) *IntCmd
	LPop(key string) *StringCmd
	LPush(key string, values ...interface{}) *IntCmd
	LPushX(key string, values ...interface{}) *IntCmd
	LRange(key string, start, stop int64) *StringSliceCmd
	LRem(key string, count int64, value interface{}) *IntCmd
	LSet(key string, index int64, value interface{}) *StatusCmd
	LTrim(key string, start, stop int64) *StatusCmd
	RPop(key string) *StringCmd
	RPopLPush(source, destination string) *StringCmd
	RPush(key string, values ...interface{}) *IntCmd
	RPushX(key string, values ...interface{}) *IntCmd
	SAdd(key string, members ...interface{}) *IntCmd
	SCard(key string) *IntCmd
	SDiff(keys ...string) *StringSliceCmd
	SDiffStore(destination string, keys ...string) *IntCmd
	SInter(keys ...string) *StringSliceCmd
	SInterStore(destination string, keys ...string) *IntCmd
	SIsMember(key string, member interface{}) *BoolCmd
	SMembers(key string) *StringSliceCmd
	SMembersMap(key string) *StringStructMapCmd
	SMove(source, destination string, member interface{}) *BoolCmd
	SPop(key string) *StringCmd
	SPopN(key string, count int64) *StringSliceCmd
	SRandMember(key string) *StringCmd
	SRandMemberN(key string, count int64) *StringSliceCmd
	SRem(key string, members ...interface{}) *IntCmd
	SUnion(keys ...string) *StringSliceCmd
	SUnionStore(destination string, keys ...string) *IntCmd
	XAdd(a *XAddArgs) *StringCmd
	XDel(stream string, ids ...string) *IntCmd
	XLen(stream string) *IntCmd
	XRange(stream, start, stop string) *XMessageSliceCmd
	XRangeN(stream, start, stop string, count int64) *XMessageSliceCmd
	XRevRange(stream string, start, stop string) *XMessageSliceCmd
	XRevRangeN(stream string, start, stop string, count int64) *XMessageSliceCmd
	XRead(a *XReadArgs) *XStreamSliceCmd
	XReadStreams(streams ...string) *XStreamSliceCmd
	XGroupCreate(stream, group, start string) *StatusCmd
	XGroupCreateMkStream(stream, group, start string) *StatusCmd
	XGroupSetID(stream, group, start string) *StatusCmd
	XGroupDestroy(stream, group string) *IntCmd
	XGroupDelConsumer(stream, group, consumer string) *IntCmd
	XReadGroup(a *XReadGroupArgs) *XStreamSliceCmd
	XAck(stream, group string, ids ...string) *IntCmd
	XPending(stream, group string) *XPendingCmd
	XPendingExt(a *XPendingExtArgs) *XPendingExtCmd
	XClaim(a *XClaimArgs) *XMessageSliceCmd
	XClaimJustID(a *XClaimArgs) *StringSliceCmd
	XTrim(key string, maxLen int64) *IntCmd
	XTrimApprox(key string, maxLen int64) *IntCmd
	XInfoGroups(key string) *XInfoGroupsCmd
	BZPopMax(timeout time.Duration, keys ...string) *ZWithKeyCmd
	BZPopMin(timeout time.Duration, keys ...string) *ZWithKeyCmd
	ZAdd(key string, members ...*Z) *IntCmd
	ZAddNX(key string, members ...*Z) *IntCmd
	ZAddXX(key string, members ...*Z) *IntCmd
	ZAddCh(key string, members ...*Z) *IntCmd
	ZAddNXCh(key string, members ...*Z) *IntCmd
	ZAddXXCh(key string, members ...*Z) *IntCmd
	ZIncr(key string, member *Z) *FloatCmd
	ZIncrNX(key string, member *Z) *FloatCmd
	ZIncrXX(key string, member *Z) *FloatCmd
	ZCard(key string) *IntCmd
	ZCount(key, min, max string) *IntCmd
	ZLexCount(key, min, max string) *IntCmd
	ZIncrBy(key string, increment float64, member string) *FloatCmd
	ZInterStore(destination string, store *ZStore) *IntCmd
	ZPopMax(key string, count ...int64) *ZSliceCmd
	ZPopMin(key string, count ...int64) *ZSliceCmd
	ZRange(key string, start, stop int64) *StringSliceCmd
	ZRangeWithScores(key string, start, stop int64) *ZSliceCmd
	ZRangeByScore(key string, opt *ZRangeBy) *StringSliceCmd
	ZRangeByLex(key string, opt *ZRangeBy) *StringSliceCmd
	ZRangeByScoreWithScores(key string, opt *ZRangeBy) *ZSliceCmd
	ZRank(key, member string) *IntCmd
	ZRem(key string, members ...interface{}) *IntCmd
	ZRemRangeByRank(key string, start, stop int64) *IntCmd
	ZRemRangeByScore(key, min, max string) *IntCmd
	ZRemRangeByLex(key, min, max string) *IntCmd
	ZRevRange(key string, start, stop int64) *StringSliceCmd
	ZRevRangeWithScores(key string, start, stop int64) *ZSliceCmd
	ZRevRangeByScore(key string, opt *ZRangeBy) *StringSliceCmd
	ZRevRangeByLex(key string, opt *ZRangeBy) *StringSliceCmd
	ZRevRangeByScoreWithScores(key string, opt *ZRangeBy) *ZSliceCmd
	ZRevRank(key, member string) *IntCmd
	ZScore(key, member string) *FloatCmd
	ZUnionStore(dest string, store *ZStore) *IntCmd
	PFAdd(key string, els ...interface{}) *IntCmd
	PFCount(keys ...string) *IntCmd
	PFMerge(dest string, keys ...string) *StatusCmd
	BgRewriteAOF() *StatusCmd
	BgSave() *StatusCmd
	ClientKill(ipPort string) *StatusCmd
	ClientKillByFilter(keys ...string) *IntCmd
	ClientList() *StringCmd
	ClientPause(dur time.Duration) *BoolCmd
	ClientID() *IntCmd
	ConfigGet(parameter string) *SliceCmd
	ConfigResetStat() *StatusCmd
	ConfigSet(parameter, value string) *StatusCmd
	ConfigRewrite() *StatusCmd
	DBSize() *IntCmd
	FlushAll() *StatusCmd
	FlushAllAsync() *StatusCmd
	FlushDB() *StatusCmd
	FlushDBAsync() *StatusCmd
	Info(section ...string) *StringCmd
	LastSave() *IntCmd
	Save() *StatusCmd
	Shutdown() *StatusCmd
	ShutdownSave() *StatusCmd
	ShutdownNoSave() *StatusCmd
	SlaveOf(host, port string) *StatusCmd
	Time() *TimeCmd
	Eval(script string, keys []string, args ...interface{}) *Cmd
	EvalSha(sha1 string, keys []string, args ...interface{}) *Cmd
	ScriptExists(hashes ...string) *BoolSliceCmd
	ScriptFlush() *StatusCmd
	ScriptKill() *StatusCmd
	ScriptLoad(script string) *StringCmd
	DebugObject(key string) *StringCmd
	Publish(channel string, message interface{}) *IntCmd
	PubSubChannels(pattern string) *StringSliceCmd
	PubSubNumSub(channels ...string) *StringIntMapCmd
	PubSubNumPat() *IntCmd
	ClusterSlots() *ClusterSlotsCmd
	ClusterNodes() *StringCmd
	ClusterMeet(host, port string) *StatusCmd
	ClusterForget(nodeID string) *StatusCmd
	ClusterReplicate(nodeID string) *StatusCmd
	ClusterResetSoft() *StatusCmd
	ClusterResetHard() *StatusCmd
	ClusterInfo() *StringCmd
	ClusterKeySlot(key string) *IntCmd
	ClusterGetKeysInSlot(slot int, count int) *StringSliceCmd
	ClusterCountFailureReports(nodeID string) *IntCmd
	ClusterCountKeysInSlot(slot int) *IntCmd
	ClusterDelSlots(slots ...int) *StatusCmd
	ClusterDelSlotsRange(min, max int) *StatusCmd
	ClusterSaveConfig() *StatusCmd
	ClusterSlaves(nodeID string) *StringSliceCmd
	ClusterFailover() *StatusCmd
	ClusterAddSlots(slots ...int) *StatusCmd
	ClusterAddSlotsRange(min, max int) *StatusCmd
	GeoAdd(key string, geoLocation ...*GeoLocation) *IntCmd
	GeoPos(key string, members ...string) *GeoPosCmd
	GeoRadius(key string, longitude, latitude float64, query *GeoRadiusQuery) *GeoLocationCmd
	GeoRadiusStore(key string, longitude, latitude float64, query *GeoRadiusQuery) *IntCmd
	GeoRadiusByMember(key, member string, query *GeoRadiusQuery) *GeoLocationCmd
	GeoRadiusByMemberStore(key, member string, query *GeoRadiusQuery) *IntCmd
	GeoDist(key string, member1, member2, unit string) *FloatCmd
	GeoHash(key string, members ...string) *StringSliceCmd
	ReadOnly() *StatusCmd
	ReadWrite() *StatusCmd
	MemoryUsage(key string, samples ...int) *IntCmd
}

type StatefulCmdable interface {
	Cmdable
	Auth(password string) *StatusCmd
	AuthACL(username, password string) *StatusCmd
	Select(index int) *StatusCmd
	SwapDB(index1, index2 int) *StatusCmd
	ClientSetName(name string) *BoolCmd
}

var _ Cmdable = (*Client)(nil)
var _ Cmdable = (*Tx)(nil)
var _ Cmdable = (*Ring)(nil)
var _ Cmdable = (*ClusterClient)(nil)

type cmdable func(cmd Cmder) error

type statefulCmdable func(cmd Cmder) error

//------------------------------------------------------------------------------

func (c statefulCmdable) Auth(password string) *StatusCmd {
	cmd := NewStatusCmd("auth", password)
	_ = c(cmd)
	return cmd
}

// Perform an AUTH command, using the given user and pass.
// Should be used to authenticate the current connection with one of the connections defined in the ACL list
// when connecting to a Redis 6.0 instance, or greater, that is using the Redis ACL system.
func (c statefulCmdable) AuthACL(username, password string) *StatusCmd {
	cmd := NewStatusCmd("auth", username, password)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Echo(message interface{}) *StringCmd {
	cmd := NewStringCmd("echo", message)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Ping() *StatusCmd {
	cmd := NewStatusCmd("ping")
	_ = c(cmd)
	return cmd
}

func (c cmdable) Wait(numSlaves int, timeout time.Duration) *IntCmd {
	cmd := NewIntCmd("wait", numSlaves, int(timeout/time.Millisecond))
	_ = c(cmd)
	return cmd
}

func (c cmdable) Quit() *StatusCmd {
	panic("not implemented")
}

func (c statefulCmdable) Select(index int) *StatusCmd {
	cmd := NewStatusCmd("select", index)
	_ = c(cmd)
	return cmd
}

func (c statefulCmdable) SwapDB(index1, index2 int) *StatusCmd {
	cmd := NewStatusCmd("swapdb", index1, index2)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) Command() *CommandsInfoCmd {
	cmd := NewCommandsInfoCmd("command")
	_ = c(cmd)
	return cmd
}

func (c cmdable) Del(keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "del"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Unlink(keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "unlink"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Dump(key string) *StringCmd {
	cmd := NewStringCmd("dump", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Exists(keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "exists"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Expire(key string, expiration time.Duration) *BoolCmd {
	cmd := NewBoolCmd("expire", key, formatSec(expiration))
	_ = c(cmd)
	return cmd
}

func (c cmdable) ExpireAt(key string, tm time.Time) *BoolCmd {
	cmd := NewBoolCmd("expireat", key, tm.Unix())
	_ = c(cmd)
	return cmd
}

func (c cmdable) Keys(pattern string) *StringSliceCmd {
	cmd := NewStringSliceCmd("keys", pattern)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Migrate(host, port, key string, db int, timeout time.Duration) *StatusCmd {
	cmd := NewStatusCmd(
		"migrate",
		host,
		port,
		key,
		db,
		formatMs(timeout),
	)
	cmd.setReadTimeout(timeout)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Move(key string, db int) *BoolCmd {
	cmd := NewBoolCmd("move", key, db)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ObjectRefCount(key string) *IntCmd {
	cmd := NewIntCmd("object", "refcount", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ObjectEncoding(key string) *StringCmd {
	cmd := NewStringCmd("object", "encoding", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ObjectIdleTime(key string) *DurationCmd {
	cmd := NewDurationCmd(time.Second, "object", "idletime", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Persist(key string) *BoolCmd {
	cmd := NewBoolCmd("persist", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) PExpire(key string, expiration time.Duration) *BoolCmd {
	cmd := NewBoolCmd("pexpire", key, formatMs(expiration))
	_ = c(cmd)
	return cmd
}

func (c cmdable) PExpireAt(key string, tm time.Time) *BoolCmd {
	cmd := NewBoolCmd(
		"pexpireat",
		key,
		tm.UnixNano()/int64(time.Millisecond),
	)
	_ = c(cmd)
	return cmd
}

func (c cmdable) PTTL(key string) *DurationCmd {
	cmd := NewDurationCmd(time.Millisecond, "pttl", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) RandomKey() *StringCmd {
	cmd := NewStringCmd("randomkey")
	_ = c(cmd)
	return cmd
}

func (c cmdable) Rename(key, newkey string) *StatusCmd {
	cmd := NewStatusCmd("rename", key, newkey)
	_ = c(cmd)
	return cmd
}

func (c cmdable) RenameNX(key, newkey string) *BoolCmd {
	cmd := NewBoolCmd("renamenx", key, newkey)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Restore(key string, ttl time.Duration, value string) *StatusCmd {
	cmd := NewStatusCmd(
		"restore",
		key,
		formatMs(ttl),
		value,
	)
	_ = c(cmd)
	return cmd
}

func (c cmdable) RestoreReplace(key string, ttl time.Duration, value string) *StatusCmd {
	cmd := NewStatusCmd(
		"restore",
		key,
		formatMs(ttl),
		value,
		"replace",
	)
	_ = c(cmd)
	return cmd
}

type Sort struct {
	By            string
	Offset, Count int64
	Get           []string
	Order         string
	Alpha         bool
}

func (sort *Sort) args(key string) []interface{} {
	args := []interface{}{"sort", key}
	if sort.By != "" {
		args = append(args, "by", sort.By)
	}
	if sort.Offset != 0 || sort.Count != 0 {
		args = append(args, "limit", sort.Offset, sort.Count)
	}
	for _, get := range sort.Get {
		args = append(args, "get", get)
	}
	if sort.Order != "" {
		args = append(args, sort.Order)
	}
	if sort.Alpha {
		args = append(args, "alpha")
	}
	return args
}

func (c cmdable) Sort(key string, sort *Sort) *StringSliceCmd {
	cmd := NewStringSliceCmd(sort.args(key)...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SortStore(key, store string, sort *Sort) *IntCmd {
	args := sort.args(key)
	if store != "" {
		args = append(args, "store", store)
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SortInterfaces(key string, sort *Sort) *SliceCmd {
	cmd := NewSliceCmd(sort.args(key)...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Touch(keys ...string) *IntCmd {
	args := make([]interface{}, len(keys)+1)
	args[0] = "touch"
	for i, key := range keys {
		args[i+1] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) TTL(key string) *DurationCmd {
	cmd := NewDurationCmd(time.Second, "ttl", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Type(key string) *StatusCmd {
	cmd := NewStatusCmd("type", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Scan(cursor uint64, match string, count int64) *ScanCmd {
	args := []interface{}{"scan", cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewScanCmd(c, args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SScan(key string, cursor uint64, match string, count int64) *ScanCmd {
	args := []interface{}{"sscan", key, cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewScanCmd(c, args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HScan(key string, cursor uint64, match string, count int64) *ScanCmd {
	args := []interface{}{"hscan", key, cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewScanCmd(c, args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZScan(key string, cursor uint64, match string, count int64) *ScanCmd {
	args := []interface{}{"zscan", key, cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewScanCmd(c, args...)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) Append(key, value string) *IntCmd {
	cmd := NewIntCmd("append", key, value)
	_ = c(cmd)
	return cmd
}

type BitCount struct {
	Start, End int64
}

func (c cmdable) BitCount(key string, bitCount *BitCount) *IntCmd {
	args := []interface{}{"bitcount", key}
	if bitCount != nil {
		args = append(
			args,
			bitCount.Start,
			bitCount.End,
		)
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) bitOp(op, destKey string, keys ...string) *IntCmd {
	args := make([]interface{}, 3+len(keys))
	args[0] = "bitop"
	args[1] = op
	args[2] = destKey
	for i, key := range keys {
		args[3+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) BitOpAnd(destKey string, keys ...string) *IntCmd {
	return c.bitOp("and", destKey, keys...)
}

func (c cmdable) BitOpOr(destKey string, keys ...string) *IntCmd {
	return c.bitOp("or", destKey, keys...)
}

func (c cmdable) BitOpXor(destKey string, keys ...string) *IntCmd {
	return c.bitOp("xor", destKey, keys...)
}

func (c cmdable) BitOpNot(destKey string, key string) *IntCmd {
	return c.bitOp("not", destKey, key)
}

func (c cmdable) BitPos(key string, bit int64, pos ...int64) *IntCmd {
	args := make([]interface{}, 3+len(pos))
	args[0] = "bitpos"
	args[1] = key
	args[2] = bit
	switch len(pos) {
	case 0:
	case 1:
		args[3] = pos[0]
	case 2:
		args[3] = pos[0]
		args[4] = pos[1]
	default:
		panic("too many arguments")
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) BitField(key string, args ...interface{}) *IntSliceCmd {
	a := make([]interface{}, 0, 2+len(args))
	a = append(a, "bitfield")
	a = append(a, key)
	a = append(a, args...)
	cmd := NewIntSliceCmd(a...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Decr(key string) *IntCmd {
	cmd := NewIntCmd("decr", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) DecrBy(key string, decrement int64) *IntCmd {
	cmd := NewIntCmd("decrby", key, decrement)
	_ = c(cmd)
	return cmd
}

// Redis `GET key` command. It returns redis.Nil error when key does not exist.
func (c cmdable) Get(key string) *StringCmd {
	cmd := NewStringCmd("get", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) GetBit(key string, offset int64) *IntCmd {
	cmd := NewIntCmd("getbit", key, offset)
	_ = c(cmd)
	return cmd
}

func (c cmdable) GetRange(key string, start, end int64) *StringCmd {
	cmd := NewStringCmd("getrange", key, start, end)
	_ = c(cmd)
	return cmd
}

func (c cmdable) GetSet(key string, value interface{}) *StringCmd {
	cmd := NewStringCmd("getset", key, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) Incr(key string) *IntCmd {
	cmd := NewIntCmd("incr", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) IncrBy(key string, value int64) *IntCmd {
	cmd := NewIntCmd("incrby", key, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) IncrByFloat(key string, value float64) *FloatCmd {
	cmd := NewFloatCmd("incrbyfloat", key, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) MGet(keys ...string) *SliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "mget"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

// MSet is like Set but accepts multiple values:
//   - MSet("key1", "value1", "key2", "value2")
//   - MSet([]string{"key1", "value1", "key2", "value2"})
//   - MSet(map[string]interface{}{"key1": "value1", "key2": "value2"})
func (c cmdable) MSet(values ...interface{}) *StatusCmd {
	args := make([]interface{}, 1, 1+len(values))
	args[0] = "mset"
	args = appendArgs(args, values)
	cmd := NewStatusCmd(args...)
	_ = c(cmd)
	return cmd
}

// MSetNX is like SetNX but accepts multiple values:
//   - MSetNX("key1", "value1", "key2", "value2")
//   - MSetNX([]string{"key1", "value1", "key2", "value2"})
//   - MSetNX(map[string]interface{}{"key1": "value1", "key2": "value2"})
func (c cmdable) MSetNX(values ...interface{}) *BoolCmd {
	args := make([]interface{}, 1, 1+len(values))
	args[0] = "msetnx"
	args = appendArgs(args, values)
	cmd := NewBoolCmd(args...)
	_ = c(cmd)
	return cmd
}

// Redis `SET key value [expiration]` command.
//
// Use expiration for `SETEX`-like behavior.
// Zero expiration means the key has no expiration time.
func (c cmdable) Set(key string, value interface{}, expiration time.Duration) *StatusCmd {
	args := make([]interface{}, 3, 5)
	args[0] = "set"
	args[1] = key
	args[2] = value
	if expiration > 0 {
		if usePrecise(expiration) {
			args = append(args, "px", formatMs(expiration))
		} else {
			args = append(args, "ex", formatSec(expiration))
		}
	}
	cmd := NewStatusCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SetBit(key string, offset int64, value int) *IntCmd {
	cmd := NewIntCmd(
		"setbit",
		key,
		offset,
		value,
	)
	_ = c(cmd)
	return cmd
}

// Redis `SET key value [expiration] NX` command.
//
// Zero expiration means the key has no expiration time.
func (c cmdable) SetNX(key string, value interface{}, expiration time.Duration) *BoolCmd {
	var cmd *BoolCmd
	if expiration == 0 {
		// Use old `SETNX` to support old Redis versions.
		cmd = NewBoolCmd("setnx", key, value)
	} else {
		if usePrecise(expiration) {
			cmd = NewBoolCmd("set", key, value, "px", formatMs(expiration), "nx")
		} else {
			cmd = NewBoolCmd("set", key, value, "ex", formatSec(expiration), "nx")
		}
	}
	_ = c(cmd)
	return cmd
}

// Redis `SET key value [expiration] XX` command.
//
// Zero expiration means the key has no expiration time.
func (c cmdable) SetXX(key string, value interface{}, expiration time.Duration) *BoolCmd {
	var cmd *BoolCmd
	if expiration == 0 {
		cmd = NewBoolCmd("set", key, value, "xx")
	} else {
		if usePrecise(expiration) {
			cmd = NewBoolCmd("set", key, value, "px", formatMs(expiration), "xx")
		} else {
			cmd = NewBoolCmd("set", key, value, "ex", formatSec(expiration), "xx")
		}
	}
	_ = c(cmd)
	return cmd
}

func (c cmdable) SetRange(key string, offset int64, value string) *IntCmd {
	cmd := NewIntCmd("setrange", key, offset, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) StrLen(key string) *IntCmd {
	cmd := NewIntCmd("strlen", key)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) HDel(key string, fields ...string) *IntCmd {
	args := make([]interface{}, 2+len(fields))
	args[0] = "hdel"
	args[1] = key
	for i, field := range fields {
		args[2+i] = field
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HExists(key, field string) *BoolCmd {
	cmd := NewBoolCmd("hexists", key, field)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HGet(key, field string) *StringCmd {
	cmd := NewStringCmd("hget", key, field)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HGetAll(key string) *StringStringMapCmd {
	cmd := NewStringStringMapCmd("hgetall", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HIncrBy(key, field string, incr int64) *IntCmd {
	cmd := NewIntCmd("hincrby", key, field, incr)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HIncrByFloat(key, field string, incr float64) *FloatCmd {
	cmd := NewFloatCmd("hincrbyfloat", key, field, incr)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HKeys(key string) *StringSliceCmd {
	cmd := NewStringSliceCmd("hkeys", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HLen(key string) *IntCmd {
	cmd := NewIntCmd("hlen", key)
	_ = c(cmd)
	return cmd
}

// HMGet returns the values for the specified fields in the hash stored at key.
// It returns an interface{} to distinguish between empty string and nil value.
func (c cmdable) HMGet(key string, fields ...string) *SliceCmd {
	args := make([]interface{}, 2+len(fields))
	args[0] = "hmget"
	args[1] = key
	for i, field := range fields {
		args[2+i] = field
	}
	cmd := NewSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

// HSet accepts values in following formats:
//   - HMSet("myhash", "key1", "value1", "key2", "value2")
//   - HMSet("myhash", []string{"key1", "value1", "key2", "value2"})
//   - HMSet("myhash", map[string]interface{}{"key1": "value1", "key2": "value2"})
//
// Note that it requires Redis v4 for multiple field/value pairs support.
func (c cmdable) HSet(key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "hset"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

// HMSet is a deprecated version of HSet left for compatibility with Redis 3.
func (c cmdable) HMSet(key string, values ...interface{}) *BoolCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "hmset"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewBoolCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HSetNX(key, field string, value interface{}) *BoolCmd {
	cmd := NewBoolCmd("hsetnx", key, field, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) HVals(key string) *StringSliceCmd {
	cmd := NewStringSliceCmd("hvals", key)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) BLPop(timeout time.Duration, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys)+1)
	args[0] = "blpop"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(args)-1] = formatSec(timeout)
	cmd := NewStringSliceCmd(args...)
	cmd.setReadTimeout(timeout)
	_ = c(cmd)
	return cmd
}

func (c cmdable) BRPop(timeout time.Duration, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys)+1)
	args[0] = "brpop"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(keys)+1] = formatSec(timeout)
	cmd := NewStringSliceCmd(args...)
	cmd.setReadTimeout(timeout)
	_ = c(cmd)
	return cmd
}

func (c cmdable) BRPopLPush(source, destination string, timeout time.Duration) *StringCmd {
	cmd := NewStringCmd(
		"brpoplpush",
		source,
		destination,
		formatSec(timeout),
	)
	cmd.setReadTimeout(timeout)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LIndex(key string, index int64) *StringCmd {
	cmd := NewStringCmd("lindex", key, index)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LInsert(key, op string, pivot, value interface{}) *IntCmd {
	cmd := NewIntCmd("linsert", key, op, pivot, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LInsertBefore(key string, pivot, value interface{}) *IntCmd {
	cmd := NewIntCmd("linsert", key, "before", pivot, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LInsertAfter(key string, pivot, value interface{}) *IntCmd {
	cmd := NewIntCmd("linsert", key, "after", pivot, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LLen(key string) *IntCmd {
	cmd := NewIntCmd("llen", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LPop(key string) *StringCmd {
	cmd := NewStringCmd("lpop", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LPush(key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "lpush"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LPushX(key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "lpushx"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LRange(key string, start, stop int64) *StringSliceCmd {
	cmd := NewStringSliceCmd(
		"lrange",
		key,
		start,
		stop,
	)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LRem(key string, count int64, value interface{}) *IntCmd {
	cmd := NewIntCmd("lrem", key, count, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LSet(key string, index int64, value interface{}) *StatusCmd {
	cmd := NewStatusCmd("lset", key, index, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LTrim(key string, start, stop int64) *StatusCmd {
	cmd := NewStatusCmd(
		"ltrim",
		key,
		start,
		stop,
	)
	_ = c(cmd)
	return cmd
}

func (c cmdable) RPop(key string) *StringCmd {
	cmd := NewStringCmd("rpop", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) RPopLPush(source, destination string) *StringCmd {
	cmd := NewStringCmd("rpoplpush", source, destination)
	_ = c(cmd)
	return cmd
}

func (c cmdable) RPush(key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "rpush"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) RPushX(key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "rpushx"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) SAdd(key string, members ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(members))
	args[0] = "sadd"
	args[1] = key
	args = appendArgs(args, members)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SCard(key string) *IntCmd {
	cmd := NewIntCmd("scard", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SDiff(keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "sdiff"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SDiffStore(destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "sdiffstore"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SInter(keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "sinter"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SInterStore(destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "sinterstore"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SIsMember(key string, member interface{}) *BoolCmd {
	cmd := NewBoolCmd("sismember", key, member)
	_ = c(cmd)
	return cmd
}

// Redis `SMEMBERS key` command output as a slice
func (c cmdable) SMembers(key string) *StringSliceCmd {
	cmd := NewStringSliceCmd("smembers", key)
	_ = c(cmd)
	return cmd
}

// Redis `SMEMBERS key` command output as a map
func (c cmdable) SMembersMap(key string) *StringStructMapCmd {
	cmd := NewStringStructMapCmd("smembers", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SMove(source, destination string, member interface{}) *BoolCmd {
	cmd := NewBoolCmd("smove", source, destination, member)
	_ = c(cmd)
	return cmd
}

// Redis `SPOP key` command.
func (c cmdable) SPop(key string) *StringCmd {
	cmd := NewStringCmd("spop", key)
	_ = c(cmd)
	return cmd
}

// Redis `SPOP key count` command.
func (c cmdable) SPopN(key string, count int64) *StringSliceCmd {
	cmd := NewStringSliceCmd("spop", key, count)
	_ = c(cmd)
	return cmd
}

// Redis `SRANDMEMBER key` command.
func (c cmdable) SRandMember(key string) *StringCmd {
	cmd := NewStringCmd("srandmember", key)
	_ = c(cmd)
	return cmd
}

// Redis `SRANDMEMBER key count` command.
func (c cmdable) SRandMemberN(key string, count int64) *StringSliceCmd {
	cmd := NewStringSliceCmd("srandmember", key, count)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SRem(key string, members ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(members))
	args[0] = "srem"
	args[1] = key
	args = appendArgs(args, members)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SUnion(keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "sunion"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SUnionStore(destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "sunionstore"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

type XAddArgs struct {
	Stream       string
	MaxLen       int64 // MAXLEN N
	MaxLenApprox int64 // MAXLEN ~ N
	ID           string
	Values       map[string]interface{}
}

func (c cmdable) XAdd(a *XAddArgs) *StringCmd {
	args := make([]interface{}, 0, 6+len(a.Values)*2)
	args = append(args, "xadd")
	args = append(args, a.Stream)
	if a.MaxLen > 0 {
		args = append(args, "maxlen", a.MaxLen)
	} else if a.MaxLenApprox > 0 {
		args = append(args, "maxlen", "~", a.MaxLenApprox)
	}
	if a.ID != "" {
		args = append(args, a.ID)
	} else {
		args = append(args, "*")
	}
	for k, v := range a.Values {
		args = append(args, k)
		args = append(args, v)
	}

	cmd := NewStringCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XDel(stream string, ids ...string) *IntCmd {
	args := []interface{}{"xdel", stream}
	for _, id := range ids {
		args = append(args, id)
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XLen(stream string) *IntCmd {
	cmd := NewIntCmd("xlen", stream)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XRange(stream, start, stop string) *XMessageSliceCmd {
	cmd := NewXMessageSliceCmd("xrange", stream, start, stop)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XRangeN(stream, start, stop string, count int64) *XMessageSliceCmd {
	cmd := NewXMessageSliceCmd("xrange", stream, start, stop, "count", count)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XRevRange(stream, start, stop string) *XMessageSliceCmd {
	cmd := NewXMessageSliceCmd("xrevrange", stream, start, stop)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XRevRangeN(stream, start, stop string, count int64) *XMessageSliceCmd {
	cmd := NewXMessageSliceCmd("xrevrange", stream, start, stop, "count", count)
	_ = c(cmd)
	return cmd
}

type XReadArgs struct {
	Streams []string // list of streams and ids, e.g. stream1 stream2 id1 id2
	Count   int64
	Block   time.Duration
}

func (c cmdable) XRead(a *XReadArgs) *XStreamSliceCmd {
	args := make([]interface{}, 0, 5+len(a.Streams))
	args = append(args, "xread")
	if a.Count > 0 {
		args = append(args, "count")
		args = append(args, a.Count)
	}
	if a.Block >= 0 {
		args = append(args, "block")
		args = append(args, int64(a.Block/time.Millisecond))
	}

	args = append(args, "streams")
	for _, s := range a.Streams {
		args = append(args, s)
	}

	cmd := NewXStreamSliceCmd(args...)
	if a.Block >= 0 {
		cmd.setReadTimeout(a.Block)
	}
	_ = c(cmd)
	return cmd
}

func (c cmdable) XReadStreams(streams ...string) *XStreamSliceCmd {
	return c.XRead(&XReadArgs{
		Streams: streams,
		Block:   -1,
	})
}

func (c cmdable) XGroupCreate(stream, group, start string) *StatusCmd {
	cmd := NewStatusCmd("xgroup", "create", stream, group, start)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XGroupCreateMkStream(stream, group, start string) *StatusCmd {
	cmd := NewStatusCmd("xgroup", "create", stream, group, start, "mkstream")
	_ = c(cmd)
	return cmd
}

func (c cmdable) XGroupSetID(stream, group, start string) *StatusCmd {
	cmd := NewStatusCmd("xgroup", "setid", stream, group, start)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XGroupDestroy(stream, group string) *IntCmd {
	cmd := NewIntCmd("xgroup", "destroy", stream, group)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XGroupDelConsumer(stream, group, consumer string) *IntCmd {
	cmd := NewIntCmd("xgroup", "delconsumer", stream, group, consumer)
	_ = c(cmd)
	return cmd
}

type XReadGroupArgs struct {
	Group    string
	Consumer string
	Streams  []string // list of streams and ids, e.g. stream1 stream2 id1 id2
	Count    int64
	Block    time.Duration
	NoAck    bool
}

func (c cmdable) XReadGroup(a *XReadGroupArgs) *XStreamSliceCmd {
	args := make([]interface{}, 0, 8+len(a.Streams))
	args = append(args, "xreadgroup", "group", a.Group, a.Consumer)
	if a.Count > 0 {
		args = append(args, "count", a.Count)
	}
	if a.Block >= 0 {
		args = append(args, "block", int64(a.Block/time.Millisecond))
	}
	if a.NoAck {
		args = append(args, "noack")
	}
	args = append(args, "streams")
	for _, s := range a.Streams {
		args = append(args, s)
	}

	cmd := NewXStreamSliceCmd(args...)
	if a.Block >= 0 {
		cmd.setReadTimeout(a.Block)
	}
	_ = c(cmd)
	return cmd
}

func (c cmdable) XAck(stream, group string, ids ...string) *IntCmd {
	args := []interface{}{"xack", stream, group}
	for _, id := range ids {
		args = append(args, id)
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XPending(stream, group string) *XPendingCmd {
	cmd := NewXPendingCmd("xpending", stream, group)
	_ = c(cmd)
	return cmd
}

type XPendingExtArgs struct {
	Stream   string
	Group    string
	Start    string
	End      string
	Count    int64
	Consumer string
}

func (c cmdable) XPendingExt(a *XPendingExtArgs) *XPendingExtCmd {
	args := make([]interface{}, 0, 7)
	args = append(args, "xpending", a.Stream, a.Group, a.Start, a.End, a.Count)
	if a.Consumer != "" {
		args = append(args, a.Consumer)
	}
	cmd := NewXPendingExtCmd(args...)
	_ = c(cmd)
	return cmd
}

type XClaimArgs struct {
	Stream   string
	Group    string
	Consumer string
	MinIdle  time.Duration
	Messages []string
}

func (c cmdable) XClaim(a *XClaimArgs) *XMessageSliceCmd {
	args := xClaimArgs(a)
	cmd := NewXMessageSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XClaimJustID(a *XClaimArgs) *StringSliceCmd {
	args := xClaimArgs(a)
	args = append(args, "justid")
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func xClaimArgs(a *XClaimArgs) []interface{} {
	args := make([]interface{}, 0, 4+len(a.Messages))
	args = append(args,
		"xclaim",
		a.Stream,
		a.Group, a.Consumer,
		int64(a.MinIdle/time.Millisecond))
	for _, id := range a.Messages {
		args = append(args, id)
	}
	return args
}

func (c cmdable) XTrim(key string, maxLen int64) *IntCmd {
	cmd := NewIntCmd("xtrim", key, "maxlen", maxLen)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XTrimApprox(key string, maxLen int64) *IntCmd {
	cmd := NewIntCmd("xtrim", key, "maxlen", "~", maxLen)
	_ = c(cmd)
	return cmd
}

func (c cmdable) XInfoGroups(key string) *XInfoGroupsCmd {
	cmd := NewXInfoGroupsCmd(key)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

// Z represents sorted set member.
type Z struct {
	Score  float64
	Member interface{}
}

// ZWithKey represents sorted set member including the name of the key where it was popped.
type ZWithKey struct {
	Z
	Key string
}

// ZStore is used as an arg to ZInterStore and ZUnionStore.
type ZStore struct {
	Keys    []string
	Weights []float64
	// Can be SUM, MIN or MAX.
	Aggregate string
}

// Redis `BZPOPMAX key [key ...] timeout` command.
func (c cmdable) BZPopMax(timeout time.Duration, keys ...string) *ZWithKeyCmd {
	args := make([]interface{}, 1+len(keys)+1)
	args[0] = "bzpopmax"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(args)-1] = formatSec(timeout)
	cmd := NewZWithKeyCmd(args...)
	cmd.setReadTimeout(timeout)
	_ = c(cmd)
	return cmd
}

// Redis `BZPOPMIN key [key ...] timeout` command.
func (c cmdable) BZPopMin(timeout time.Duration, keys ...string) *ZWithKeyCmd {
	args := make([]interface{}, 1+len(keys)+1)
	args[0] = "bzpopmin"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(args)-1] = formatSec(timeout)
	cmd := NewZWithKeyCmd(args...)
	cmd.setReadTimeout(timeout)
	_ = c(cmd)
	return cmd
}

func (c cmdable) zAdd(a []interface{}, n int, members ...*Z) *IntCmd {
	for i, m := range members {
		a[n+2*i] = m.Score
		a[n+2*i+1] = m.Member
	}
	cmd := NewIntCmd(a...)
	_ = c(cmd)
	return cmd
}

// Redis `ZADD key score member [score member ...]` command.
func (c cmdable) ZAdd(key string, members ...*Z) *IntCmd {
	const n = 2
	a := make([]interface{}, n+2*len(members))
	a[0], a[1] = "zadd", key
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key NX score member [score member ...]` command.
func (c cmdable) ZAddNX(key string, members ...*Z) *IntCmd {
	const n = 3
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2] = "zadd", key, "nx"
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key XX score member [score member ...]` command.
func (c cmdable) ZAddXX(key string, members ...*Z) *IntCmd {
	const n = 3
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2] = "zadd", key, "xx"
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key CH score member [score member ...]` command.
func (c cmdable) ZAddCh(key string, members ...*Z) *IntCmd {
	const n = 3
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2] = "zadd", key, "ch"
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key NX CH score member [score member ...]` command.
func (c cmdable) ZAddNXCh(key string, members ...*Z) *IntCmd {
	const n = 4
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2], a[3] = "zadd", key, "nx", "ch"
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key XX CH score member [score member ...]` command.
func (c cmdable) ZAddXXCh(key string, members ...*Z) *IntCmd {
	const n = 4
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2], a[3] = "zadd", key, "xx", "ch"
	return c.zAdd(a, n, members...)
}

func (c cmdable) zIncr(a []interface{}, n int, members ...*Z) *FloatCmd {
	for i, m := range members {
		a[n+2*i] = m.Score
		a[n+2*i+1] = m.Member
	}
	cmd := NewFloatCmd(a...)
	_ = c(cmd)
	return cmd
}

// Redis `ZADD key INCR score member` command.
func (c cmdable) ZIncr(key string, member *Z) *FloatCmd {
	const n = 3
	a := make([]interface{}, n+2)
	a[0], a[1], a[2] = "zadd", key, "incr"
	return c.zIncr(a, n, member)
}

// Redis `ZADD key NX INCR score member` command.
func (c cmdable) ZIncrNX(key string, member *Z) *FloatCmd {
	const n = 4
	a := make([]interface{}, n+2)
	a[0], a[1], a[2], a[3] = "zadd", key, "incr", "nx"
	return c.zIncr(a, n, member)
}

// Redis `ZADD key XX INCR score member` command.
func (c cmdable) ZIncrXX(key string, member *Z) *FloatCmd {
	const n = 4
	a := make([]interface{}, n+2)
	a[0], a[1], a[2], a[3] = "zadd", key, "incr", "xx"
	return c.zIncr(a, n, member)
}

func (c cmdable) ZCard(key string) *IntCmd {
	cmd := NewIntCmd("zcard", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZCount(key, min, max string) *IntCmd {
	cmd := NewIntCmd("zcount", key, min, max)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZLexCount(key, min, max string) *IntCmd {
	cmd := NewIntCmd("zlexcount", key, min, max)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZIncrBy(key string, increment float64, member string) *FloatCmd {
	cmd := NewFloatCmd("zincrby", key, increment, member)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZInterStore(destination string, store *ZStore) *IntCmd {
	args := make([]interface{}, 3+len(store.Keys))
	args[0] = "zinterstore"
	args[1] = destination
	args[2] = len(store.Keys)
	for i, key := range store.Keys {
		args[3+i] = key
	}
	if len(store.Weights) > 0 {
		args = append(args, "weights")
		for _, weight := range store.Weights {
			args = append(args, weight)
		}
	}
	if store.Aggregate != "" {
		args = append(args, "aggregate", store.Aggregate)
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZPopMax(key string, count ...int64) *ZSliceCmd {
	args := []interface{}{
		"zpopmax",
		key,
	}

	switch len(count) {
	case 0:
		break
	case 1:
		args = append(args, count[0])
	default:
		panic("too many arguments")
	}

	cmd := NewZSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZPopMin(key string, count ...int64) *ZSliceCmd {
	args := []interface{}{
		"zpopmin",
		key,
	}

	switch len(count) {
	case 0:
		break
	case 1:
		args = append(args, count[0])
	default:
		panic("too many arguments")
	}

	cmd := NewZSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) zRange(key string, start, stop int64, withScores bool) *StringSliceCmd {
	args := []interface{}{
		"zrange",
		key,
		start,
		stop,
	}
	if withScores {
		args = append(args, "withscores")
	}
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRange(key string, start, stop int64) *StringSliceCmd {
	return c.zRange(key, start, stop, false)
}

func (c cmdable) ZRangeWithScores(key string, start, stop int64) *ZSliceCmd {
	cmd := NewZSliceCmd("zrange", key, start, stop, "withscores")
	_ = c(cmd)
	return cmd
}

type ZRangeBy struct {
	Min, Max      string
	Offset, Count int64
}

func (c cmdable) zRangeBy(zcmd, key string, opt *ZRangeBy, withScores bool) *StringSliceCmd {
	args := []interface{}{zcmd, key, opt.Min, opt.Max}
	if withScores {
		args = append(args, "withscores")
	}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"limit",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRangeByScore(key string, opt *ZRangeBy) *StringSliceCmd {
	return c.zRangeBy("zrangebyscore", key, opt, false)
}

func (c cmdable) ZRangeByLex(key string, opt *ZRangeBy) *StringSliceCmd {
	return c.zRangeBy("zrangebylex", key, opt, false)
}

func (c cmdable) ZRangeByScoreWithScores(key string, opt *ZRangeBy) *ZSliceCmd {
	args := []interface{}{"zrangebyscore", key, opt.Min, opt.Max, "withscores"}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"limit",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewZSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRank(key, member string) *IntCmd {
	cmd := NewIntCmd("zrank", key, member)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRem(key string, members ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(members))
	args[0] = "zrem"
	args[1] = key
	args = appendArgs(args, members)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRemRangeByRank(key string, start, stop int64) *IntCmd {
	cmd := NewIntCmd(
		"zremrangebyrank",
		key,
		start,
		stop,
	)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRemRangeByScore(key, min, max string) *IntCmd {
	cmd := NewIntCmd("zremrangebyscore", key, min, max)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRemRangeByLex(key, min, max string) *IntCmd {
	cmd := NewIntCmd("zremrangebylex", key, min, max)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRevRange(key string, start, stop int64) *StringSliceCmd {
	cmd := NewStringSliceCmd("zrevrange", key, start, stop)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRevRangeWithScores(key string, start, stop int64) *ZSliceCmd {
	cmd := NewZSliceCmd("zrevrange", key, start, stop, "withscores")
	_ = c(cmd)
	return cmd
}

func (c cmdable) zRevRangeBy(zcmd, key string, opt *ZRangeBy) *StringSliceCmd {
	args := []interface{}{zcmd, key, opt.Max, opt.Min}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"limit",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRevRangeByScore(key string, opt *ZRangeBy) *StringSliceCmd {
	return c.zRevRangeBy("zrevrangebyscore", key, opt)
}

func (c cmdable) ZRevRangeByLex(key string, opt *ZRangeBy) *StringSliceCmd {
	return c.zRevRangeBy("zrevrangebylex", key, opt)
}

func (c cmdable) ZRevRangeByScoreWithScores(key string, opt *ZRangeBy) *ZSliceCmd {
	args := []interface{}{"zrevrangebyscore", key, opt.Max, opt.Min, "withscores"}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"limit",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewZSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZRevRank(key, member string) *IntCmd {
	cmd := NewIntCmd("zrevrank", key, member)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZScore(key, member string) *FloatCmd {
	cmd := NewFloatCmd("zscore", key, member)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ZUnionStore(dest string, store *ZStore) *IntCmd {
	args := make([]interface{}, 3+len(store.Keys))
	args[0] = "zunionstore"
	args[1] = dest
	args[2] = len(store.Keys)
	for i, key := range store.Keys {
		args[3+i] = key
	}
	if len(store.Weights) > 0 {
		args = append(args, "weights")
		for _, weight := range store.Weights {
			args = append(args, weight)
		}
	}
	if store.Aggregate != "" {
		args = append(args, "aggregate", store.Aggregate)
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) PFAdd(key string, els ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(els))
	args[0] = "pfadd"
	args[1] = key
	args = appendArgs(args, els)
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) PFCount(keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "pfcount"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) PFMerge(dest string, keys ...string) *StatusCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "pfmerge"
	args[1] = dest
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewStatusCmd(args...)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) BgRewriteAOF() *StatusCmd {
	cmd := NewStatusCmd("bgrewriteaof")
	_ = c(cmd)
	return cmd
}

func (c cmdable) BgSave() *StatusCmd {
	cmd := NewStatusCmd("bgsave")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClientKill(ipPort string) *StatusCmd {
	cmd := NewStatusCmd("client", "kill", ipPort)
	_ = c(cmd)
	return cmd
}

// ClientKillByFilter is new style synx, while the ClientKill is old
// CLIENT KILL <option> [value] ... <option> [value]
func (c cmdable) ClientKillByFilter(keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "client"
	args[1] = "kill"
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClientList() *StringCmd {
	cmd := NewStringCmd("client", "list")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClientPause(dur time.Duration) *BoolCmd {
	cmd := NewBoolCmd("client", "pause", formatMs(dur))
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClientID() *IntCmd {
	cmd := NewIntCmd("client", "id")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClientUnblock(id int64) *IntCmd {
	cmd := NewIntCmd("client", "unblock", id)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClientUnblockWithError(id int64) *IntCmd {
	cmd := NewIntCmd("client", "unblock", id, "error")
	_ = c(cmd)
	return cmd
}

// ClientSetName assigns a name to the connection.
func (c statefulCmdable) ClientSetName(name string) *BoolCmd {
	cmd := NewBoolCmd("client", "setname", name)
	_ = c(cmd)
	return cmd
}

// ClientGetName returns the name of the connection.
func (c cmdable) ClientGetName() *StringCmd {
	cmd := NewStringCmd("client", "getname")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ConfigGet(parameter string) *SliceCmd {
	cmd := NewSliceCmd("config", "get", parameter)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ConfigResetStat() *StatusCmd {
	cmd := NewStatusCmd("config", "resetstat")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ConfigSet(parameter, value string) *StatusCmd {
	cmd := NewStatusCmd("config", "set", parameter, value)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ConfigRewrite() *StatusCmd {
	cmd := NewStatusCmd("config", "rewrite")
	_ = c(cmd)
	return cmd
}

// Deperecated. Use DBSize instead.
func (c cmdable) DbSize() *IntCmd {
	return c.DBSize()
}

func (c cmdable) DBSize() *IntCmd {
	cmd := NewIntCmd("dbsize")
	_ = c(cmd)
	return cmd
}

func (c cmdable) FlushAll() *StatusCmd {
	cmd := NewStatusCmd("flushall")
	_ = c(cmd)
	return cmd
}

func (c cmdable) FlushAllAsync() *StatusCmd {
	cmd := NewStatusCmd("flushall", "async")
	_ = c(cmd)
	return cmd
}

func (c cmdable) FlushDB() *StatusCmd {
	cmd := NewStatusCmd("flushdb")
	_ = c(cmd)
	return cmd
}

func (c cmdable) FlushDBAsync() *StatusCmd {
	cmd := NewStatusCmd("flushdb", "async")
	_ = c(cmd)
	return cmd
}

func (c cmdable) Info(section ...string) *StringCmd {
	args := []interface{}{"info"}
	if len(section) > 0 {
		args = append(args, section[0])
	}
	cmd := NewStringCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) LastSave() *IntCmd {
	cmd := NewIntCmd("lastsave")
	_ = c(cmd)
	return cmd
}

func (c cmdable) Save() *StatusCmd {
	cmd := NewStatusCmd("save")
	_ = c(cmd)
	return cmd
}

func (c cmdable) shutdown(modifier string) *StatusCmd {
	var args []interface{}
	if modifier == "" {
		args = []interface{}{"shutdown"}
	} else {
		args = []interface{}{"shutdown", modifier}
	}
	cmd := NewStatusCmd(args...)
	_ = c(cmd)
	if err := cmd.Err(); err != nil {
		if err == io.EOF {
			// Server quit as expected.
			cmd.err = nil
		}
	} else {
		// Server did not quit. String reply contains the reason.
		cmd.err = errors.New(cmd.val)
		cmd.val = ""
	}
	return cmd
}

func (c cmdable) Shutdown() *StatusCmd {
	return c.shutdown("")
}

func (c cmdable) ShutdownSave() *StatusCmd {
	return c.shutdown("save")
}

func (c cmdable) ShutdownNoSave() *StatusCmd {
	return c.shutdown("nosave")
}

func (c cmdable) SlaveOf(host, port string) *StatusCmd {
	cmd := NewStatusCmd("slaveof", host, port)
	_ = c(cmd)
	return cmd
}

func (c cmdable) SlowLog() {
	panic("not implemented")
}

func (c cmdable) Sync() {
	panic("not implemented")
}

func (c cmdable) Time() *TimeCmd {
	cmd := NewTimeCmd("time")
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) Eval(script string, keys []string, args ...interface{}) *Cmd {
	cmdArgs := make([]interface{}, 3+len(keys), 3+len(keys)+len(args))
	cmdArgs[0] = "eval"
	cmdArgs[1] = script
	cmdArgs[2] = len(keys)
	for i, key := range keys {
		cmdArgs[3+i] = key
	}
	cmdArgs = appendArgs(cmdArgs, args)
	cmd := NewCmd(cmdArgs...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) EvalSha(sha1 string, keys []string, args ...interface{}) *Cmd {
	cmdArgs := make([]interface{}, 3+len(keys), 3+len(keys)+len(args))
	cmdArgs[0] = "evalsha"
	cmdArgs[1] = sha1
	cmdArgs[2] = len(keys)
	for i, key := range keys {
		cmdArgs[3+i] = key
	}
	cmdArgs = appendArgs(cmdArgs, args)
	cmd := NewCmd(cmdArgs...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ScriptExists(hashes ...string) *BoolSliceCmd {
	args := make([]interface{}, 2+len(hashes))
	args[0] = "script"
	args[1] = "exists"
	for i, hash := range hashes {
		args[2+i] = hash
	}
	cmd := NewBoolSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ScriptFlush() *StatusCmd {
	cmd := NewStatusCmd("script", "flush")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ScriptKill() *StatusCmd {
	cmd := NewStatusCmd("script", "kill")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ScriptLoad(script string) *StringCmd {
	cmd := NewStringCmd("script", "load", script)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) DebugObject(key string) *StringCmd {
	cmd := NewStringCmd("debug", "object", key)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

// Publish posts the message to the channel.
func (c cmdable) Publish(channel string, message interface{}) *IntCmd {
	cmd := NewIntCmd("publish", channel, message)
	_ = c(cmd)
	return cmd
}

func (c cmdable) PubSubChannels(pattern string) *StringSliceCmd {
	args := []interface{}{"pubsub", "channels"}
	if pattern != "*" {
		args = append(args, pattern)
	}
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) PubSubNumSub(channels ...string) *StringIntMapCmd {
	args := make([]interface{}, 2+len(channels))
	args[0] = "pubsub"
	args[1] = "numsub"
	for i, channel := range channels {
		args[2+i] = channel
	}
	cmd := NewStringIntMapCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) PubSubNumPat() *IntCmd {
	cmd := NewIntCmd("pubsub", "numpat")
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) ClusterSlots() *ClusterSlotsCmd {
	cmd := NewClusterSlotsCmd("cluster", "slots")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterNodes() *StringCmd {
	cmd := NewStringCmd("cluster", "nodes")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterMeet(host, port string) *StatusCmd {
	cmd := NewStatusCmd("cluster", "meet", host, port)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterForget(nodeID string) *StatusCmd {
	cmd := NewStatusCmd("cluster", "forget", nodeID)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterReplicate(nodeID string) *StatusCmd {
	cmd := NewStatusCmd("cluster", "replicate", nodeID)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterResetSoft() *StatusCmd {
	cmd := NewStatusCmd("cluster", "reset", "soft")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterResetHard() *StatusCmd {
	cmd := NewStatusCmd("cluster", "reset", "hard")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterInfo() *StringCmd {
	cmd := NewStringCmd("cluster", "info")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterKeySlot(key string) *IntCmd {
	cmd := NewIntCmd("cluster", "keyslot", key)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterGetKeysInSlot(slot int, count int) *StringSliceCmd {
	cmd := NewStringSliceCmd("cluster", "getkeysinslot", slot, count)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterCountFailureReports(nodeID string) *IntCmd {
	cmd := NewIntCmd("cluster", "count-failure-reports", nodeID)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterCountKeysInSlot(slot int) *IntCmd {
	cmd := NewIntCmd("cluster", "countkeysinslot", slot)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterDelSlots(slots ...int) *StatusCmd {
	args := make([]interface{}, 2+len(slots))
	args[0] = "cluster"
	args[1] = "delslots"
	for i, slot := range slots {
		args[2+i] = slot
	}
	cmd := NewStatusCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterDelSlotsRange(min, max int) *StatusCmd {
	size := max - min + 1
	slots := make([]int, size)
	for i := 0; i < size; i++ {
		slots[i] = min + i
	}
	return c.ClusterDelSlots(slots...)
}

func (c cmdable) ClusterSaveConfig() *StatusCmd {
	cmd := NewStatusCmd("cluster", "saveconfig")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterSlaves(nodeID string) *StringSliceCmd {
	cmd := NewStringSliceCmd("cluster", "slaves", nodeID)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ReadOnly() *StatusCmd {
	cmd := NewStatusCmd("readonly")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ReadWrite() *StatusCmd {
	cmd := NewStatusCmd("readwrite")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterFailover() *StatusCmd {
	cmd := NewStatusCmd("cluster", "failover")
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterAddSlots(slots ...int) *StatusCmd {
	args := make([]interface{}, 2+len(slots))
	args[0] = "cluster"
	args[1] = "addslots"
	for i, num := range slots {
		args[2+i] = num
	}
	cmd := NewStatusCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) ClusterAddSlotsRange(min, max int) *StatusCmd {
	size := max - min + 1
	slots := make([]int, size)
	for i := 0; i < size; i++ {
		slots[i] = min + i
	}
	return c.ClusterAddSlots(slots...)
}

//------------------------------------------------------------------------------

func (c cmdable) GeoAdd(key string, geoLocation ...*GeoLocation) *IntCmd {
	args := make([]interface{}, 2+3*len(geoLocation))
	args[0] = "geoadd"
	args[1] = key
	for i, eachLoc := range geoLocation {
		args[2+3*i] = eachLoc.Longitude
		args[2+3*i+1] = eachLoc.Latitude
		args[2+3*i+2] = eachLoc.Name
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}

// GeoRadius is a read-only GEORADIUS_RO command.
func (c cmdable) GeoRadius(key string, longitude, latitude float64, query *GeoRadiusQuery) *GeoLocationCmd {
	cmd := NewGeoLocationCmd(query, "georadius_ro", key, longitude, latitude)
	if query.Store != "" || query.StoreDist != "" {
		cmd.SetErr(errors.New("GeoRadius does not support Store or StoreDist"))
		return cmd
	}
	_ = c(cmd)
	return cmd
}

// GeoRadiusStore is a writing GEORADIUS command.
func (c cmdable) GeoRadiusStore(key string, longitude, latitude float64, query *GeoRadiusQuery) *IntCmd {
	args := geoLocationArgs(query, "georadius", key, longitude, latitude)
	cmd := NewIntCmd(args...)
	if query.Store == "" && query.StoreDist == "" {
		cmd.SetErr(errors.New("GeoRadiusStore requires Store or StoreDist"))
		return cmd
	}
	_ = c(cmd)
	return cmd
}

// GeoRadius is a read-only GEORADIUSBYMEMBER_RO command.
func (c cmdable) GeoRadiusByMember(key, member string, query *GeoRadiusQuery) *GeoLocationCmd {
	cmd := NewGeoLocationCmd(query, "georadiusbymember_ro", key, member)
	if query.Store != "" || query.StoreDist != "" {
		cmd.SetErr(errors.New("GeoRadiusByMember does not support Store or StoreDist"))
		return cmd
	}
	_ = c(cmd)
	return cmd
}

// GeoRadiusByMemberStore is a writing GEORADIUSBYMEMBER command.
func (c cmdable) GeoRadiusByMemberStore(key, member string, query *GeoRadiusQuery) *IntCmd {
	args := geoLocationArgs(query, "georadiusbymember", key, member)
	cmd := NewIntCmd(args...)
	if query.Store == "" && query.StoreDist == "" {
		cmd.SetErr(errors.New("GeoRadiusByMemberStore requires Store or StoreDist"))
		return cmd
	}
	_ = c(cmd)
	return cmd
}

func (c cmdable) GeoDist(key string, member1, member2, unit string) *FloatCmd {
	if unit == "" {
		unit = "km"
	}
	cmd := NewFloatCmd("geodist", key, member1, member2, unit)
	_ = c(cmd)
	return cmd
}

func (c cmdable) GeoHash(key string, members ...string) *StringSliceCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "geohash"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewStringSliceCmd(args...)
	_ = c(cmd)
	return cmd
}

func (c cmdable) GeoPos(key string, members ...string) *GeoPosCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "geopos"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewGeoPosCmd(args...)
	_ = c(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) MemoryUsage(key string, samples ...int) *IntCmd {
	args := []interface{}{"memory", "usage", key}
	if len(samples) > 0 {
		if len(samples) != 1 {
			panic("MemoryUsage expects single sample count")
		}
		args = append(args, "SAMPLES", samples[0])
	}
	cmd := NewIntCmd(args...)
	_ = c(cmd)
	return cmd
}
