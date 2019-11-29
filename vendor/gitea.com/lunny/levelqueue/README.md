# levelqueue

Level queue is a simple queue golang library base on go-leveldb.

[![Build Status](https://drone.gitea.com/api/badges/lunny/levelqueue/status.svg)](https://drone.gitea.com/lunny/levelqueue)  [![](http://gocover.io/_badge/gitea.com/lunny/levelqueue)](http://gocover.io/gitea.com/lunny/levelqueue)
[![](https://goreportcard.com/badge/gitea.com/lunny/levelqueue)](https://goreportcard.com/report/gitea.com/lunny/levelqueue)

## Installation

```
go get gitea.com/lunny/levelqueue
```

## Usage

```Go
queue, err := levelqueue.Open("./queue")

err = queue.RPush([]byte("test"))

// pop an element from left of the queue
data, err = queue.LPop()

// if handle success, element will be pop, otherwise it will be keep
queue.LHandle(func(dt []byte) error{
    return nil
})
```