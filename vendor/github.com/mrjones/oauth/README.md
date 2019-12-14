OAuth 1.0 Library for [Go](http://golang.org)
========================

[![GoDoc](http://godoc.org/github.com/mrjones/oauth?status.png)](http://godoc.org/github.com/mrjones/oauth)

[![CircleCI](https://circleci.com/gh/mrjones/oauth/tree/master.svg?style=svg)](https://circleci.com/gh/mrjones/oauth/tree/master)

(If you need an OAuth 2.0 library, check out: https://godoc.org/golang.org/x/oauth2)

Developing your own apps, with this library
-------------------------------------------

* First, install the library

        go get github.com/mrjones/oauth

* Then, check out the comments in oauth.go

* Or, have a look at the examples:

    * Netflix

            go run examples/netflix/netflix.go --consumerkey [key] --consumersecret [secret] --appname [appname]

    * Twitter
    
        Command line:

            go run examples/twitter/twitter.go --consumerkey [key] --consumersecret [secret]
            
        Or, in the browser (using an HTTP server):
        
            go run examples/twitterserver/twitterserver.go --consumerkey [key] --consumersecret [secret] --port 8888        

    * The Google Latitude example is broken, now that Google uses OAuth 2.0

Contributing to this library
----------------------------

* Please install the pre-commit hook, which will run tests, and go-fmt before committing.

        ln -s $PWD/pre-commit.sh .git/hooks/pre-commit

* Running tests and building is as you'd expect:

        go test *.go
        go build *.go




