# A smart client for couchbase in go

This is a *unoffical* version of a Couchbase Golang client. If you are
looking for the *Offical* Couchbase Golang client please see
    [CB-go])[https://github.com/couchbaselabs/gocb].

This is an evolving package, but does provide a useful interface to a
[couchbase](http://www.couchbase.com/) server including all of the
pool/bucket discovery features, compatible key distribution with other
clients, and vbucket motion awareness so application can continue to
operate during rebalances.

It also supports view querying with source node randomization so you
don't bang on all one node to do all the work.

## Install

    go get github.com/couchbase/go-couchbase

## Example

    c, err := couchbase.Connect("http://dev-couchbase.example.com:8091/")
    if err != nil {
    	log.Fatalf("Error connecting:  %v", err)
    }

    pool, err := c.GetPool("default")
    if err != nil {
    	log.Fatalf("Error getting pool:  %v", err)
    }

    bucket, err := pool.GetBucket("default")
    if err != nil {
    	log.Fatalf("Error getting bucket:  %v", err)
    }

    bucket.Set("someKey", 0, []string{"an", "example", "list"})
