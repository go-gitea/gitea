// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ssh

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"github.com/gliderlabs/ssh"
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

func shellEscape(args []string) string {
	var data = make([]string, len(args))
	for k, v := range args {
		data[k] = v

		// This is a borderline dirty hack. It's designed to only escape
		// strings which *might* need to be escaped. It uses a very
		// limited character set so everything that needs to be quoted
		// will be but we can ignore some simple cases to make it easier
		// to parse in the ssh hook. We allow alpha-numeric characters
		// along with ., _, and -
		idx := strings.IndexFunc(v, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune(".-_", r)
		})
		if idx > -1 || v == "" {
			data[k] = "'" + strings.Replace(data[k], "'", "'\"'\"'", -1) + "'"
		}
	}
	return strings.Join(data, " ")
}

func sessionHandler(session ssh.Session) {
	keyID := session.Context().Value(giteaKeyID).(int64)

	command := shellEscape(session.Command())

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
		log.Error(3, "SSH: StdoutPipe: %v", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error(3, "SSH: StderrPipe: %v", err)
		return
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error(3, "SSH: StdinPipe: %v", err)
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	if err = cmd.Start(); err != nil {
		log.Error(3, "SSH: Start: %v", err)
		return
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, session)
	}()

	go func() {
		defer wg.Done()
		io.Copy(session, stdout)
	}()

	go func() {
		defer wg.Done()
		io.Copy(session.Stderr(), stderr)
	}()

	// Ensure all the output has been written before we wait on the command
	// to exit.
	wg.Wait()

	// Wait for the command to exit and log any errors we get
	err = cmd.Wait()
	if err != nil {
		log.Error(3, "SSH: Wait: %v", err)
	}

	session.Exit(getExitStatusFromError(err))
}

func publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	if ctx.User() != setting.SSH.BuiltinServerUser {
		return false
	}

	pkey, err := models.SearchPublicKeyByContent(strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))))
	if err != nil {
		log.Error(3, "SearchPublicKeyByContent: %v", err)
		return false
	}

	ctx.SetValue(giteaKeyID, pkey.ID)

	return true
}

// Listen starts a SSH server listens on given port.
func Listen(host string, port int, ciphers []string, keyExchanges []string, macs []string) {
	// TODO: Handle ciphers, keyExchanges, and macs

	srv := ssh.Server{
		Addr:             host + ":" + com.ToStr(port),
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
			log.Error(4, "Failed to create dir %s: %v", filePath, err)
		}

		_, stderr, err := com.ExecCmd("ssh-keygen", "-f", keyPath, "-t", "rsa", "-N", "")
		if err != nil {
			log.Fatal(4, "Failed to generate private key: %v - %s", err, stderr)
		}
		log.Trace("SSH: New private key is generated: %s", keyPath)
	}

	srv.SetOption(ssh.HostKeyFile(keyPath))

	go srv.ListenAndServe()
}
