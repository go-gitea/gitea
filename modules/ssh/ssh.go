// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/modules/generate"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	gossh "golang.org/x/crypto/ssh"
)

const giteaPermissionExtensionKeyID = "gitea-perm-ext-key-id"

func getExitStatusFromError(err error) int {
	if err == nil {
		return 0
	}

	exitErr, ok := errors.AsType[*exec.ExitError](err)
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

func sessionHandler(session *sshSession) int {
	// the conn permissions are the ones of the key which really authenticated, see publicKeyHandler
	keyID := session.conn.Permissions.Extensions[giteaPermissionExtensionKeyID]

	log.Trace("SSH: Payload: %v", session.rawCmd)

	args := []string{"--config=" + setting.CustomConf, "serv", "key-" + keyID}
	log.Trace("SSH: Arguments: %v", args)

	ctx, cancel := context.WithCancel(session.ctx)
	defer cancel()

	gitProtocol := ""
	for _, env := range session.env {
		if strings.HasPrefix(env, "GIT_PROTOCOL=") {
			_, gitProtocol, _ = strings.Cut(env, "=")
			break
		}
	}

	cmd := exec.CommandContext(ctx, setting.AppPath, args...)
	cmd.Env = append(
		os.Environ(),
		"SSH_ORIGINAL_COMMAND="+session.rawCmd,
		"SKIP_MINWINSVC=1",
		"GIT_PROTOCOL="+gitProtocol,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("SSH: StdoutPipe: %v", err)
		return 1
	}
	defer stdout.Close()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error("SSH: StderrPipe: %v", err)
		return 1
	}
	defer stderr.Close()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error("SSH: StdinPipe: %v", err)
		return 1
	}
	defer stdin.Close()

	process.SetSysProcAttribute(cmd)

	wg := &sync.WaitGroup{}

	if err = cmd.Start(); err != nil {
		log.Error("SSH: Start: %v", err)
		return 1
	}

	go func() {
		defer stdin.Close()
		if _, err := io.Copy(stdin, session); err != nil {
			log.Error("Failed to write session to stdin. %s", err)
		}
	}()

	wg.Go(func() {
		defer stdout.Close()
		if _, err := io.Copy(session, stdout); err != nil {
			log.Error("Failed to write stdout to session. %s", err)
		}
	})

	wg.Go(func() {
		defer stderr.Close()
		if _, err := io.Copy(session.Stderr(), stderr); err != nil {
			log.Error("Failed to write stderr to session. %s", err)
		}
	})

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

	return getExitStatusFromError(err)
}

func keyPermissions(keyID int64) *gossh.Permissions {
	return &gossh.Permissions{Extensions: map[string]string{
		giteaPermissionExtensionKeyID: strconv.FormatInt(keyID, 10),
	}}
}

// publicKeyHandler only offers the candidate keys, it does not verify them. x/crypto assigns the
// returned Permissions to the ssh conn once it verified the signature for that key, so a user
// offering keys A (with a private key) and B (without one) authenticates and is served as A.
func publicKeyHandler(ctx context.Context, conn gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Handle Public Key: Fingerprint: %s from %s", gossh.FingerprintSHA256(key), conn.RemoteAddr())
	}

	if conn.User() != setting.SSH.BuiltinServerUser {
		log.Warn("Invalid SSH username %s - must use %s for all git operations via ssh", conn.User(), setting.SSH.BuiltinServerUser)
		log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())
		return nil, util.ErrPermissionDenied
	}

	// check if we have a certificate
	if cert, ok := key.(*gossh.Certificate); ok {
		if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
			log.Debug("Handle Certificate: %s Fingerprint: %s is a certificate", conn.RemoteAddr(), gossh.FingerprintSHA256(key))
		}

		if len(setting.SSH.TrustedUserCAKeys) == 0 {
			log.Warn("Certificate Rejected: No trusted certificate authorities for this server")
			log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())
			return nil, util.ErrPermissionDenied
		}

		if cert.CertType != gossh.UserCert {
			log.Warn("Certificate Rejected: Not a user certificate")
			log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())
			return nil, util.ErrPermissionDenied
		}

		// look for the exact principal
	principalLoop:
		for _, principal := range cert.ValidPrincipals {
			pkey, err := asymkey_model.SearchPublicKeyByContentExact(ctx, principal)
			if err != nil {
				if asymkey_model.IsErrKeyNotExist(err) {
					log.Debug("Principal Rejected: %s Unknown Principal: %s", conn.RemoteAddr(), principal)
					continue principalLoop
				}
				log.Error("SearchPublicKeyByContentExact: %v", err)
				return nil, util.ErrPermissionDenied
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
					log.Debug("Principal Rejected: %s Untrusted Authority Signature Fingerprint %s for Principal: %s", conn.RemoteAddr(), gossh.FingerprintSHA256(cert.SignatureKey), principal)
				}
				continue principalLoop
			}

			// validate the cert for this principal
			if err := c.CheckCert(principal, cert); err != nil {
				// User is presenting an invalid certificate - STOP any further processing
				log.Error("Invalid Certificate KeyID %s with Signature Fingerprint %s presented for Principal: %s from %s", cert.KeyId, gossh.FingerprintSHA256(cert.SignatureKey), principal, conn.RemoteAddr())
				log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())

				return nil, util.ErrPermissionDenied
			}

			if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
				log.Debug("Successfully authenticated: %s Certificate Fingerprint: %s Principal: %s", conn.RemoteAddr(), gossh.FingerprintSHA256(key), principal)
			}
			return keyPermissions(pkey.ID), nil
		}

		log.Warn("From %s Fingerprint: %s is a certificate, but no valid principals found", conn.RemoteAddr(), gossh.FingerprintSHA256(key))
		log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())
		return nil, util.ErrPermissionDenied
	}

	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Handle Public Key: %s Fingerprint: %s is not a certificate", conn.RemoteAddr(), gossh.FingerprintSHA256(key))
	}

	pkey, err := asymkey_model.SearchPublicKeyByContent(ctx, strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))))
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			log.Warn("Unknown public key: %s from %s", gossh.FingerprintSHA256(key), conn.RemoteAddr())
			log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())
			return nil, util.ErrPermissionDenied
		}
		log.Error("SearchPublicKeyByContent: %v", err)
		return nil, util.ErrPermissionDenied
	}

	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Successfully authenticated: %s Public Key Fingerprint: %s", conn.RemoteAddr(), gossh.FingerprintSHA256(key))
	}
	return keyPermissions(pkey.ID), nil
}

// sshConnectionFailed logs a failed connection
// - this mainly exists to give a nice function name in logging
func sshConnectionFailed(conn net.Conn, err error) {
	// Log the underlying error with a specific message
	log.Warn("Failed connection from %s with error: %v", conn.RemoteAddr(), err)
	// Log with the standard failed authentication from message for simpler fail2ban configuration
	log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())
}

// Listen starts an SSH server listening on given port.
func Listen(host string, port int, ciphers, keyExchanges, macs []string) {
	hostKeyFiles := make([]string, 0, len(setting.SSH.ServerHostKeys))
	for _, key := range setting.SSH.ServerHostKeys {
		_, err := os.Stat(key)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Fatal("Unable to check if %s exists. Error: %v", setting.SSH.ServerHostKeys, err)
			}
			continue
		}
		hostKeyFiles = append(hostKeyFiles, key)
	}

	if len(hostKeyFiles) == 0 {
		hostKeyDir := filepath.Dir(setting.SSH.ServerHostKeys[0])
		err := os.MkdirAll(hostKeyDir, os.ModePerm)
		if err != nil {
			log.Error("Failed to create dir %s: %v", hostKeyDir, err)
		}
		hostKeyFiles, err = InitDefaultHostKeys(hostKeyDir)
		if err != nil {
			log.Fatal("Failed to generate private key: %v", err)
		}
	}

	var hostSigners []gossh.Signer
	for _, keyFile := range hostKeyFiles {
		pemBytes, err := os.ReadFile(keyFile)
		if err == nil {
			var signer gossh.Signer
			if signer, err = gossh.ParsePrivateKey(pemBytes); err == nil {
				log.Info("Adding SSH host key: %s", keyFile)
				hostSigners = append(hostSigners, signer)
				continue
			}
		}
		log.Error("Failed to load SSH host key %s: %v", keyFile, err)
	}

	if len(hostSigners) == 0 {
		log.Fatal("No usable SSH host key, tried: %v", hostKeyFiles)
	}

	srv := &sshServer{
		addr:        net.JoinHostPort(host, strconv.Itoa(port)),
		hostSigners: hostSigners,
		config:      gossh.Config{Ciphers: ciphers, KeyExchanges: keyExchanges, MACs: macs},
	}

	go func() {
		_, _, finished := process.GetManager().AddTypedContext(graceful.GetManager().HammerContext(), "Service: Built-in SSH server", process.SystemProcessType, true)
		defer finished()
		listen(srv)
	}()
}

// GenKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
func GenKeyPair(keyPath string, keyType generate.SSHKeyType, bits int) error {
	publicKey, privateKeyPEM, err := generate.NewSSHKey(keyType, bits)
	if err != nil {
		return err
	}

	public := gossh.MarshalAuthorizedKey(publicKey)
	privateKeyBuf := &bytes.Buffer{}
	err = pem.Encode(privateKeyBuf, privateKeyPEM)
	if err != nil {
		return err
	}

	err = os.WriteFile(keyPath, privateKeyBuf.Bytes(), 0o600)
	if err != nil {
		return err
	}

	return os.WriteFile(keyPath+".pub", public, 0o644)
}

// InitDefaultHostKeys mirrors how ssh-keygen -A operates
// it runs checks if public and private keys are already defined and creates new ones if not present
// key naming does not follow the OpenSSH convention due to existing settings being gitea.{KeyType} so generation follows gitea convention
func InitDefaultHostKeys(path string) (keyFiles []string, _ error) {
	var errs []error
	keyTypes := []generate.SSHKeyType{generate.SSHKeyRSA, generate.SSHKeyECDSA, generate.SSHKeyED25519}
	for _, keyType := range keyTypes {
		keyPath := filepath.Join(path, "gitea."+string(keyType))
		_, errStatPriv := os.Stat(keyPath)
		if errStatPriv != nil {
			err := GenKeyPair(keyPath, keyType, 0)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
		keyFiles = append(keyFiles, keyPath)
	}
	return keyFiles, errors.Join(errs...)
}
