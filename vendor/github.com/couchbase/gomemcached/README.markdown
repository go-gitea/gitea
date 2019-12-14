# gomemcached

This is a memcached binary protocol toolkit in [go][go].

It provides client and server functionality as well as a little sample
server showing how I might make a server if I valued purity over
performance.

## Server Design

<div>
  <img src="http://dustin.github.com/images/gomemcached.png"
       alt="overview" style="float: right"/>
</div>

The basic design can be seen in [gocache].  A [storage
server][storage] is run as a goroutine that receives a `MCRequest` on
a channel, and then issues an `MCResponse` to a channel contained
within the request.

Each connection is a separate goroutine, of course, and is responsible
for all IO for that connection until the connection drops or the
`dataServer` decides it's stupid and sends a fatal response back over
the channel.

There is currently no work at all in making the thing perform (there
are specific areas I know need work).  This is just my attempt to
learn the language somewhat.

[go]: http://golang.org/
[gocache]: gomemcached/blob/master/gocache/gocache.go
[storage]: gomemcached/blob/master/gocache/mc_storage.go
