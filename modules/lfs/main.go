package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	contentMediaType = "application/vnd.git-lfs"
	metaMediaType    = contentMediaType + "+json"
	version          = "0.3.0"
)

var (
	logger = NewKVLogger(os.Stdout)
)

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func wrapHttps(l net.Listener, cert, key string) (net.Listener, error) {
	var err error

	config := &tls.Config{}

	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	netListener := l.(*TrackingListener).Listener

	tlsListener := tls.NewListener(tcpKeepAliveListener{netListener.(*net.TCPListener)}, config)
	return tlsListener, nil
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-v" {
		fmt.Println(version)
		os.Exit(0)
	}

	var listener net.Listener

	tl, err := NewTrackingListener(Config.Listen)
	if err != nil {
		logger.Fatal(kv{"fn": "main", "err": "Could not create listener: " + err.Error()})
	}

	listener = tl

	if Config.IsHTTPS() {
		logger.Log(kv{"fn": "main", "msg": "Using https"})
		listener, err = wrapHttps(tl, Config.Cert, Config.Key)
		if err != nil {
			logger.Fatal(kv{"fn": "main", "err": "Could not create https listener: " + err.Error()})
		}
	}

	metaStore, err := NewMetaStore(Config.MetaDB)
	if err != nil {
		logger.Fatal(kv{"fn": "main", "err": "Could not open the meta store: " + err.Error()})
	}

	contentStore, err := NewContentStore(Config.ContentPath)
	if err != nil {
		logger.Fatal(kv{"fn": "main", "err": "Could not open the content store: " + err.Error()})
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func(c chan os.Signal, listener net.Listener) {
		for {
			sig := <-c
			switch sig {
			case syscall.SIGHUP: // Graceful shutdown
				tl.Close()
			}
		}
	}(c, tl)

	logger.Log(kv{"fn": "main", "msg": "listening", "pid": os.Getpid(), "addr": Config.Listen, "version": version})

	app := NewApp(contentStore, metaStore)
	app.Serve(listener)
	tl.WaitForChildren()
}
