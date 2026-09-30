// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"

	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	gossh "golang.org/x/crypto/ssh"
)

// sshServer rejects everything but the "session" channel and its "exec" and "shell" requests,
// because pty, subsystems and forwarding are of no use to a git client.
type sshServer struct {
	addr        string
	hostSigners []gossh.Signer
	config      gossh.Config
}

type sshSession struct {
	gossh.Channel
	conn   *gossh.ServerConn
	ctx    context.Context
	rawCmd string
	env    []string
}

func (srv *sshServer) newServerConfig(ctx context.Context) *gossh.ServerConfig {
	config := &gossh.ServerConfig{
		Config: srv.config,
		PublicKeyCallback: func(conn gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			return publicKeyHandler(ctx, conn, key)
		},
	}
	for _, signer := range srv.hostSigners {
		config.AddHostKey(signer) // keeps only the newest key per algorithm
	}
	return config
}

func listen(srv *sshServer) {
	gracefulServer := graceful.NewServer("tcp", srv.addr, "SSH")
	gracefulServer.PerWriteTimeout = setting.SSH.PerWriteTimeout
	gracefulServer.PerWritePerKbTimeout = setting.SSH.PerWritePerKbTimeout

	err := gracefulServer.ListenAndServe(srv.serve, setting.SSH.UseProxyProtocol)
	if err != nil {
		select {
		case <-graceful.GetManager().IsShutdown():
			log.Error("Failed to start SSH server: %v", err)
		default:
			log.Fatal("Failed to start SSH server: %v", err)
		}
	}
	log.Info("SSH Listener: %s Closed", srv.addr)
}

// serve is a graceful.ServeFunction
func (srv *sshServer) serve(listener net.Listener) error {
	var acceptDelay time.Duration
	for {
		conn, err := listener.Accept()
		if err != nil {
			// out of file descriptors or an aborted handshake, both recover on their own
			if !errors.Is(err, syscall.EMFILE) && !errors.Is(err, syscall.ENFILE) && !errors.Is(err, syscall.ECONNABORTED) {
				return err
			}
			acceptDelay = min(max(2*acceptDelay, 5*time.Millisecond), time.Second)
			log.Warn("SSH: Accept failed, retrying in %s: %v", acceptDelay, err)
			time.Sleep(acceptDelay)
			continue
		}
		acceptDelay = 0
		go srv.handleConn(conn)
	}
}

func (srv *sshServer) handleConn(netConn net.Conn) {
	ctx, cancel := context.WithCancel(graceful.GetManager().HammerContext())
	defer cancel()
	defer netConn.Close()

	conn, chans, reqs, err := gossh.NewServerConn(netConn, srv.newServerConfig(ctx))
	if err != nil {
		sshConnectionFailed(netConn, err)
		return
	}

	go gossh.DiscardRequests(reqs)
	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(gossh.UnknownChannelType, "unsupported channel type")
			continue
		}
		go handleSessionChannel(ctx, conn, newChan)
	}
}

func handleSessionChannel(ctx context.Context, conn *gossh.ServerConn, newChan gossh.NewChannel) {
	channel, reqs, err := newChan.Accept()
	if err != nil {
		log.Error("SSH: Accept session channel: %v", err)
		return
	}
	defer channel.Close()

	session := &sshSession{Channel: channel, conn: conn, ctx: ctx}
	for req := range reqs {
		switch req.Type {
		case "env":
			var env struct{ Key, Value string }
			if gossh.Unmarshal(req.Payload, &env) != nil {
				_ = req.Reply(false, nil)
				continue
			}
			session.env = append(session.env, env.Key+"="+env.Value)
			_ = req.Reply(true, nil)
		case "exec", "shell":
			var payload struct{ Value string } // a "shell" carries no payload, it runs "gitea serv" without a command
			if req.Type == "exec" && gossh.Unmarshal(req.Payload, &payload) != nil {
				_ = req.Reply(false, nil)
				continue
			}
			session.rawCmd = payload.Value
			_ = req.Reply(true, nil)
			go gossh.DiscardRequests(reqs) // the client keeps sending while the command runs
			status := struct{ Status uint32 }{uint32(sessionHandler(session))}
			if _, err := channel.SendRequest("exit-status", false, gossh.Marshal(&status)); err != nil {
				log.Error("SSH: Send exit-status: %v", err)
			}
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}
