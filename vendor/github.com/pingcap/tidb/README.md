![logo](./docs/logo_with_text.png)
[![Build Status](https://travis-ci.org/pingcap/tidb.svg?branch=master)](https://travis-ci.org/pingcap/tidb)
## What is TiDB?

TiDB is a distributed SQL database.
Inspired by the design of Google [F1](http://research.google.com/pubs/pub41344.html), TiDB supports the best features of both traditional RDBMS and NoSQL.

- __Horizontal scalability__  
Grow TiDB as your business grows. You can increase the capacity simply by adding more machines.

- __Asynchronous schema changes__  
Evolve TiDB schemas as your requirement evolves. You can add new columns and indices without stopping or affecting the on-going operations.

- __Consistent distributed transactions__  
Think TiDB as a single-machine RDBMS. You can start a transaction that crosses multiple machines without worrying about consistency. TiDB makes your application code simple and robust.

- __Compatible with MySQL protocol__  
Use TiDB as MySQL. You can replace MySQL with TiDB to power your application without changing a single line of code in most cases.

- __Written in Go__  
Enjoy TiDB as much as we love Go. We believe Go code is both easy and enjoyable to work with. Go makes us improve TiDB fast and makes it easy to dive into the codebase.


- __NewSQL over HBase__  
Turn HBase into NewSQL database

- __Multiple storage engine support__  
Power TiDB with your most favorite engines. TiDB supports many popular storage engines in single-machine mode. You can choose from GolevelDB, LevelDB, RocksDB, LMDB, BoltDB and even more to come.

## Status

TiDB is at its early age and under heavy development, all of the features mentioned above are fully implemented.

__Please do not use it in production.__

## Roadmap

Read the [Roadmap](./docs/ROADMAP.md).

## Quick start

Read the [Quick Start](./docs/QUICKSTART.md)

## Architecture

![architecture](./docs/architecture.png)

## Contributing
Contributions are welcomed and greatly appreciated. See [CONTRIBUTING.md](CONTRIBUTING.md)
for details on submitting patches and the contribution workflow.

## Follow us

Twitter: [@PingCAP](https://twitter.com/PingCAP)

## License
TiDB is under the Apache 2.0 license. See the [LICENSE](./LICENSES/LICENSE) file for details.

## Acknowledgments
- Thanks [cznic](https://github.com/cznic) for providing some great open source tools.
- Thanks [Xiaomi](https://github.com/XiaoMi/themis) for providing the great open source project.
- Thanks [HBase](https://hbase.apache.org), [GolevelDB](https://github.com/syndtr/goleveldb), [LMDB](https://github.com/LMDB/lmdb), [BoltDB](https://github.com/boltdb/bolt) and [RocksDB](https://github.com/facebook/rocksdb) for their powerful storage engines.
