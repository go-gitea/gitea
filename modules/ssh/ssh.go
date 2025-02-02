// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// The ssh auth overall works like this:
// NewServerConn:
//	serverHandshake+serverAuthenticate:
//		PublicKeyCallback:
//			PublicKeyHandler (our code):
//				reset(ctx.Permissions) and set ctx.Permissions.giteaKeyID = keyID
//		pubKey.Verify
//		return ctx.Permissions // only reaches here, the pub key is really authenticated
//	set conn.Permissions from serverAuthenticate
//  sessionHandler(conn)
//
// Then sessionHandler should only use the "verified keyID" from the original ssh conn, but not the ctx one.
// Otherwise, if a user provides 2 keys A (a correct one) and B (public key matches but no private key),
// then only A succeeds to authenticate, sessionHandler will see B's keyID
//
// After x/crypto >= 0.31.0 (fix CVE-2024-45337), the PublicKeyCallback will be called again for the verified key,
// it mitigates the misuse for most cases, it's still good for us to make sure we don't rely on that mitigation
// and do not misuse the PublicKeyCallback: we should only use the verified keyID from the verified ssh conn.

const giteaPermissionExtensionKeyID = "gitea-perm-ext-key-id"

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

// sessionPartial is the private struct from "gliderlabs/ssh/session.go"
// We need to read the original "conn" field from "ssh.Session interface" which contains the "*session pointer"
// https://github.com/gliderlabs/ssh/blob/d137aad99cd6f2d9495bfd98c755bec4e5dffb8c/session.go#L109-L113
// If upstream fixes the problem and/or changes the struct, we need to follow.
// If the struct mismatches, the builtin ssh server will fail during integration tests.
type sessionPartial struct {
	sync.Mutex
	gossh.Channel
	conn *gossh.ServerConn
}

func ptr[T any](intf any) *T {
	// https://pkg.go.dev/unsafe#Pointer
	// (1) Conversion of a *T1 to Pointer to *T2.
	// Provided that T2 is no larger than T1 and that the two share an equivalent memory layout,
	// this conversion allows reinterpreting data of one type as data of another type.
	v := reflect.ValueOf(intf)
	p := v.UnsafePointer()
	return (*T)(p)
}

func sessionHandler(session ssh.Session) {
	// here can't use session.Permissions() because it only uses the value from ctx, which might not be the authenticated one.
	// so we must use the original ssh conn, which always contains the correct (verified) keyID.
	sshSession := ptr[sessionPartial](session)
	keyID := sshSession.conn.Permissions.Extensions[giteaPermissionExtensionKeyID]

	command := session.RawCommand()

	log.Trace("SSH: Payload: %v", command)

	args := []string{"--config=" + setting.CustomConf, "serv", "key-" + keyID}
	log.Trace("SSH: Arguments: %v", args)

	ctx, cancel := context.WithCancel(session.Context())
	defer cancel()

	gitProtocol := ""
	for _, env := range session.Environ() {
		if strings.HasPrefix(env, "GIT_PROTOCOL=") {
			_, gitProtocol, _ = strings.Cut(env, "=")
			break
		}
	}

	cmd := exec.CommandContext(ctx, setting.AppPath, args...)
	cmd.Env = append(
		os.Environ(),
		"SSH_ORIGINAL_COMMAND="+command,
		"SKIP_MINWINSVC=1",
		"GIT_PROTOCOL="+gitProtocol,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("SSH: StdoutPipe: %v", err)
		return
	}
	defer stdout.Close()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error("SSH: StderrPipe: %v", err)
		return
	}
	defer stderr.Close()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error("SSH: StdinPipe: %v", err)
		return
	}
	defer stdin.Close()

	process.SetSysProcAttribute(cmd)

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
		defer stdout.Close()
		if _, err := io.Copy(session, stdout); err != nil {
			log.Error("Failed to write stdout to session. %s", err)
		}
	}()

	go func() {
		defer wg.Done()
		defer stderr.Close()
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
		// Cannot use errors.Is here because ExitError doesn't implement Is
		// Thus errors.Is will do equality test NOT type comparison
		if _, ok := err.(*exec.ExitError); !ok {
			log.Error("SSH: Wait: %v", err)
		}
	}

	if err := session.Exit(getExitStatusFromError(err)); err != nil && !errors.Is(err, io.EOF) {
		log.Error("Session failed to exit. %s", err)
	}
}

func publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	// The publicKeyHandler (PublicKeyCallback) only helps to provide the candidate keys to authenticate,
	// It does NOT really verify here, so we could only record the related information here.
	// After authentication (Verify), the "Permissions" will be assigned to the ssh conn,
	// then we can use it in the "session handler"

	// first, reset the ctx permissions (just like https://github.com/gliderlabs/ssh/pull/243 does)
	// it shouldn't be reused across different ssh conn (sessions), each pub key should have its own "Permissions"
	ctx.Permissions().Permissions = &gossh.Permissions{}
	setPermExt := func(keyID int64) {
		ctx.Permissions().Permissions.Extensions = map[string]string{
			giteaPermissionExtensionKeyID: fmt.Sprint(keyID),
		}
	}

	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Handle Public Key: Fingerprint: %s from %s", gossh.FingerprintSHA256(key), ctx.RemoteAddr())
	}

	if ctx.User() != setting.SSH.BuiltinServerUser {
		log.Warn("Invalid SSH username %s - must use %s for all git operations via ssh", ctx.User(), setting.SSH.BuiltinServerUser)
		log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
		return false
	}

	// check if we have a certificate
	if cert, ok := key.(*gossh.Certificate); ok {
		if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
			log.Debug("Handle Certificate: %s Fingerprint: %s is a certificate", ctx.RemoteAddr(), gossh.FingerprintSHA256(key))
		}

		if len(setting.SSH.TrustedUserCAKeys) == 0 {
			log.Warn("Certificate Rejected: No trusted certificate authorities for this server")
			log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
			return false
		}

		if cert.CertType != gossh.UserCert {
			log.Warn("Certificate Rejected: Not a user certificate")
			log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
			return false
		}

		// look for the exact principal
	principalLoop:
		for _, principal := range cert.ValidPrincipals {
			pkey, err := asymkey_model.SearchPublicKeyByContentExact(ctx, principal)
			if err != nil {
				if asymkey_model.IsErrKeyNotExist(err) {
					log.Debug("Principal Rejected: %s Unknown Principal: %s", ctx.RemoteAddr(), principal)
					continue principalLoop
				}
				log.Error("SearchPublicKeyByContentExact: %v", err)
				return false
			}

			c := &gossh.CertChecker{
				IsUserAuthority: func(auth gossh.PublicKey) bool {
					marshaled := auth.Marshal()
					for _, k := range setting.SSH.TrustedUserCAKeysParsed {
						if bytes.Equal(marshaled, k.Marshal()) {
							return true
						}
					}

					return false
				},
			}

			// check the CA of the cert
			if !c.IsUserAuthority(cert.SignatureKey) {
				if log.IsDebug() {
					log.Debug("Principal Rejected: %s Untrusted Authority Signature Fingerprint %s for Principal: %s", ctx.RemoteAddr(), gossh.FingerprintSHA256(cert.SignatureKey), principal)
				}
				continue principalLoop
			}

			// validate the cert for this principal
			if err := c.CheckCert(principal, cert); err != nil {
				// User is presenting an invalid certificate - STOP any further processing
				log.Error("Invalid Certificate KeyID %s with Signature Fingerprint %s presented for Principal: %s from %s", cert.KeyId, gossh.FingerprintSHA256(cert.SignatureKey), principal, ctx.RemoteAddr())
				log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())

				return false
			}

			if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
				log.Debug("Successfully authenticated: %s Certificate Fingerprint: %s Principal: %s", ctx.RemoteAddr(), gossh.FingerprintSHA256(key), principal)
			}
			setPermExt(pkey.ID)
			return true
		}

		log.Warn("From %s Fingerprint: %s is a certificate, but no valid principals found", ctx.RemoteAddr(), gossh.FingerprintSHA256(key))
		log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
		return false
	}

	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Handle Public Key: %s Fingerprint: %s is not a certificate", ctx.RemoteAddr(), gossh.FingerprintSHA256(key))
	}

	pkey, err := asymkey_model.SearchPublicKeyByContent(ctx, strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))))
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			log.Warn("Unknown public key: %s from %s", gossh.FingerprintSHA256(key), ctx.RemoteAddr())
			log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
			return false
		}
		log.Error("SearchPublicKeyByContent: %v", err)
		return false
	}

	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Successfully authenticated: %s Public Key Fingerprint: %s", ctx.RemoteAddr(), gossh.FingerprintSHA256(key))
	}
	setPermExt(pkey.ID)
	return true
}

// sshConnectionFailed logs a failed connection
// -  this mainly exists to give a nice function name in logging
func sshConnectionFailed(conn net.Conn, err error) {
	// Log the underlying error with a specific message
	log.Warn("Failed connection from %s with error: %v", conn.RemoteAddr(), err)
	// Log with the standard failed authentication from message for simpler fail2ban configuration
	log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())
}

// Listen starts a SSH server listens on given port.
func Listen(host string, port int, ciphers, keyExchanges, macs []string) {
	srv := ssh.Server{
		Addr:             net.JoinHostPort(host, strconv.Itoa(port)),
		PublicKeyHandler: publicKeyHandler,
		Handler:          sessionHandler,
		ServerConfigCallback: func(ctx ssh.Context) *gossh.ServerConfig {
			config := &gossh.ServerConfig{}
			config.KeyExchanges = keyExchanges
			config.MACs = macs
			config.Ciphers = ciphers
			return config
		},
		ConnectionFailedCallback: sshConnectionFailed,
		// We need to explicitly disable the PtyCallback so text displays
		// properly.
		PtyCallback: func(ctx ssh.Context, pty ssh.Pty) bool {
			return false
		},
	}

	keys := make([]string, 0, len(setting.SSH.ServerHostKeys))
	for _, key := range setting.SSH.ServerHostKeys {
		isExist, err := util.IsExist(key)
		if err != nil {
			log.Fatal("Unable to check if %s exists. Error: %v", setting.SSH.ServerHostKeys, err)
		}
		if isExist {
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		filePath := filepath.Dir(setting.SSH.ServerHostKeys[0])

		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			log.Error("Failed to create dir %s: %v", filePath, err)
		}

		err := GenKeyPair(setting.SSH.ServerHostKeys[0])
		if err != nil {
			log.Fatal("Failed to generate private key: %v", err)
		}
		log.Trace("New private key is generated: %s", setting.SSH.ServerHostKeys[0])
		keys = append(keys, setting.SSH.ServerHostKeys[0])
	}

	for _, key := range keys {
		log.Info("Adding SSH host key: %s", key)
		err := srv.SetOption(ssh.HostKeyFile(key))
		if err != nil {
			log.Error("Failed to set Host Key. %s", err)
		}
	}

	go func() {
		_, _, finished := process.GetManager().AddTypedContext(graceful.GetManager().HammerContext(), "Service: Built-in SSH server", process.SystemProcessType, true)
		defer finished()
		listen(&srv)
	}()
}

// GenKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
func GenKeyPair(keyPath string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	f, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
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
	p, err := os.OpenFile(keyPath+".pub", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
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
