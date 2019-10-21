// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/gliderlabs/ssh"
	"github.com/unknwon/com"
	gossh "golang.org/x/crypto/ssh"
)

type contextKey string

const giteaKeyID = contextKey("gitea-key-id")

func getExitStatusFromError(err error) int {
	if err == nil {
		return 0
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return 1
	}

	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		// This is a fallback and should at least let us return something useful
		// when running on Windows, even if it isn't completely accurate.
		if exitErr.Success() {
			return 0
		}

		return 1
	}

	return waitStatus.ExitStatus()
}

func sessionHandler(session ssh.Session) {
	keyID := session.Context().Value(giteaKeyID).(int64)

	command := session.RawCommand()

	log.Trace("SSH: Payload: %v", command)

	args := []string{"serv", "key-" + com.ToStr(keyID), "--config=" + setting.CustomConf}
	log.Trace("SSH: Arguments: %v", args)
	cmd := exec.Command(setting.AppPath, args...)
	cmd.Env = append(
		os.Environ(),
		"SSH_ORIGINAL_COMMAND="+command,
		"SKIP_MINWINSVC=1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("SSH: StdoutPipe: %v", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error("SSH: StderrPipe: %v", err)
		return
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error("SSH: StdinPipe: %v", err)
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	if err = cmd.Start(); err != nil {
		log.Error("SSH: Start: %v", err)
		return
	}

	go func() {
		defer stdin.Close()
		if _, err := io.Copy(stdin, session); err != nil {
			log.Error("Failed to write session to stdin. %s", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(session, stdout); err != nil {
			log.Error("Failed to write stdout to session. %s", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(session.Stderr(), stderr); err != nil {
			log.Error("Failed to write stderr to session. %s", err)
		}
	}()

	// Ensure all the output has been written before we wait on the command
	// to exit.
	wg.Wait()

	// Wait for the command to exit and log any errors we get
	err = cmd.Wait()
	if err != nil {
		log.Error("SSH: Wait: %v", err)
	}

	if err := session.Exit(getExitStatusFromError(err)); err != nil {
		log.Error("Session failed to exit. %s", err)
	}
}

func publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	if ctx.User() != setting.SSH.BuiltinServerUser {
		return false
	}

	pkey, err := models.SearchPublicKeyByContent(strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))))
	if err != nil {
		log.Error("SearchPublicKeyByContent: %v", err)
		return false
	}

	ctx.SetValue(giteaKeyID, pkey.ID)

	return true
}

// Listen starts a SSH server listens on given port.
func Listen(host string, port int, ciphers []string, keyExchanges []string, macs []string) {
	// TODO: Handle ciphers, keyExchanges, and macs

	srv := ssh.Server{
		Addr:             fmt.Sprintf("%s:%d", host, port),
		PublicKeyHandler: publicKeyHandler,
		Handler:          sessionHandler,

		// We need to explicitly disable the PtyCallback so text displays
		// properly.
		PtyCallback: func(ctx ssh.Context, pty ssh.Pty) bool {
			return false
		},
	}

	keyPath := filepath.Join(setting.AppDataPath, "ssh/gogs.rsa")
	if !com.IsExist(keyPath) {
		filePath := filepath.Dir(keyPath)

		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			log.Error("Failed to create dir %s: %v", filePath, err)
		}

		err := GenKeyPair(keyPath)
		if err != nil {
			log.Fatal("Failed to generate private key: %v", err)
		}
		log.Trace("New private key is generated: %s", keyPath)
	}

	err := srv.SetOption(ssh.HostKeyFile(keyPath))
	if err != nil {
		log.Error("Failed to set Host Key. %s", err)
	}

	go listen(&srv)

}

// GenKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
func GenKeyPair(keyPath string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	f, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Error("Close: %v", err)
		}
	}()

	if err := pem.Encode(f, privateKeyPEM); err != nil {
		return err
	}

	// generate public key
	pub, err := gossh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}

	public := gossh.MarshalAuthorizedKey(pub)
	p, err := os.OpenFile(keyPath+".pub", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err = p.Close(); err != nil {
			log.Error("Close: %v", err)
		}
	}()
	_, err = p.Write(public)
	return err
}
