# levelqueue

Level queue is a simple queue golang library base on go-leveldb.

[![CircleCI](https://circleci.com/gh/lunny/levelqueue.svg?style=shield)](https://circleci.com/gh/lunny/levelqueue)
[![codecov](https://codecov.io/gh/lunny/levelqueue/branch/master/graph/badge.svg)](https://codecov.io/gh/lunny/levelqueue)
[![](https://goreportcard.com/badge/github.com/lunny/levelqueue)](https://goreportcard.com/report/github.com/lunny/levelqueue) 

## Installation

```
go get github.com/lunny/levelqueue
```

## Usage

```Go
queue, err := levelqueue.Open("./queue")

err = queue.RPush([]byte("test"))

data, err = queue.LPop()
```