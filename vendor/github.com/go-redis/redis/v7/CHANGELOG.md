# Changelog

## v7.2

- Existing `HMSet` is renamed to `HSet` and old deprecated `HMSet` is restored for Redis 3 users.

## v7.1

- Existing `Cmd.String` is renamed to `Cmd.Text`. New `Cmd.String` implements `fmt.Stringer` interface.

## v7

- *Important*. Tx.Pipeline now returns a non-transactional pipeline. Use Tx.TxPipeline for a transactional pipeline.
- WrapProcess is replaced with more convenient AddHook that has access to context.Context.
- WithContext now can not be used to create a shallow copy of the client.
- New methods ProcessContext, DoContext, and ExecContext.
- Client respects Context.Deadline when setting net.Conn deadline.
- Client listens on Context.Done while waiting for a connection from the pool and returns an error when context context is cancelled.
- Add PubSub.ChannelWithSubscriptions that sends `*Subscription` in addition to `*Message` to allow detecting reconnections.
- `time.Time` is now marshalled in RFC3339 format. `rdb.Get("foo").Time()` helper is added to parse the time.
- `SetLimiter` is removed and added `Options.Limiter` instead.
- `HMSet` is deprecated as of Redis v4.

## v6.15

- Cluster and Ring pipelines process commands for each node in its own goroutine.

## 6.14

- Added Options.MinIdleConns.
- Added Options.MaxConnAge.
- PoolStats.FreeConns is renamed to PoolStats.IdleConns.
- Add Client.Do to simplify creating custom commands.
- Add Cmd.String, Cmd.Int, Cmd.Int64, Cmd.Uint64, Cmd.Float64, and Cmd.Bool helpers.
- Lower memory usage.

## v6.13

- Ring got new options called `HashReplicas` and `Hash`. It is recommended to set `HashReplicas = 1000` for better keys distribution between shards.
- Cluster client was optimized to use much less memory when reloading cluster state.
- PubSub.ReceiveMessage is re-worked to not use ReceiveTimeout so it does not lose data when timeout occurres. In most cases it is recommended to use PubSub.Channel instead.
- Dialer.KeepAlive is set to 5 minutes by default.

## v6.12

- ClusterClient got new option called `ClusterSlots` which allows to build cluster of normal Redis Servers that don't have cluster mode enabled. See https://godoc.org/github.com/go-redis/redis#example-NewClusterClient--ManualSetup
