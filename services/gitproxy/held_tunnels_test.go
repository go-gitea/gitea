// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeldRemoteTunnels replicates the reported fd-exhaustion reproduction at the proxy
// level: a dumb-HTTP remote that answers every request with Connection: close and then
// holds the connection open makes git open one CONNECT tunnel per object; the tunnels
// must not outlive the clone. Not parallel: it mutates the global git env.
func TestHeldRemoteTunnels(t *testing.T) {
	base := t.TempDir()

	// a repository of 200 loose blobs, each with a tag, plus a branch tip, for git's dumb HTTP protocol
	gitDir := filepath.Join(base, "repo.git")
	require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "objects", "info"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "info"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "objects", "info", "packs"), nil, 0o644))
	tree := writeLooseObject(t, gitDir, "tree", "")
	commit := writeLooseObject(t, gitDir, "commit", "tree "+tree+"\nauthor A <a@example.com> 1700000000 +0000\ncommitter A <a@example.com> 1700000000 +0000\n\ninit\n")
	var refs strings.Builder
	fmt.Fprintf(&refs, "%s\trefs/heads/main\n", commit)
	for i := range 200 {
		fmt.Fprintf(&refs, "%s\trefs/tags/t%04d\n", writeLooseObject(t, gitDir, "blob", fmt.Sprintf("object %d\n", i)), i)
	}
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "info", "refs"), []byte(refs.String()), 0o644))

	// the attacker's server: dumb HTTP over TLS, Connection: close on every response,
	// /polite.git closes the connection, /held.git holds it open without sending more
	release := make(chan struct{})
	defer close(release) // before the cleanups: attacker.Close waits for the held handlers
	attacker := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		repo := strings.TrimPrefix(r.URL.Path, "/held.git")
		repo = strings.TrimPrefix(repo, "/polite.git")
		http.ServeFile(w, r, filepath.Join(gitDir, filepath.FromSlash(path.Clean("/"+repo))))
		if strings.HasPrefix(r.URL.Path, "/held.git") {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-release
		}
	}))
	attacker.StartTLS()
	t.Cleanup(attacker.Close)
	certFile := filepath.Join(base, "attacker.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: attacker.Certificate().Raw}), 0o644))

	// the proxy, counting the upstream connections it holds open
	var held atomic.Int64
	srv := newServer(allowLoopback, testAuth, nil)
	origDial := srv.dial
	srv.dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := origDial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		held.Add(1)
		return &trackedConn{Conn: conn, onClose: func() { held.Add(-1) }}, nil
	}
	proxyAddr := startProxy(t, srv)

	exe, err := os.Executable()
	require.NoError(t, err)
	defer test.MockVariableValue(&setting.AppPath, exe)()
	defer test.MockVariableValue(&setting.Git.HomePath, base)()
	t.Cleanup(func() { gitcmd.SetExtraEnvs(nil) })
	gitcmd.SetExtraEnvs(append(gitEnvs("http://test:test@"+proxyAddr), "GIT_SSL_CAINFO="+certFile))

	mirror := func(name string) {
		_, stderr, runErr := gitcmd.NewCommand("clone", "--mirror", "-q").AddDynamicArguments("https://"+attacker.Listener.Addr().String()+"/"+name, filepath.Join(base, name)).RunStdString(t.Context())
		require.NoError(t, runErr, stderr)
	}
	waitHeld := func(want int64) {
		for deadline := time.Now().Add(5 * time.Second); ; {
			if held.Load() == want {
				return
			}
			if time.Now().After(deadline) {
				assert.Equal(t, want, held.Load())
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	t.Run("PoliteRemoteLeavesNoTunnels", func(t *testing.T) {
		mirror("polite.git")
		waitHeld(0)
	})
	t.Run("HeldRemoteLeavesNoTunnels", func(t *testing.T) {
		mirror("held.git")
		waitHeld(0)
	})
}

// writeLooseObject writes content as a loose object of the given type into the repository and returns its sha
func writeLooseObject(t *testing.T, repoDir, typ, content string) string {
	data := typ + " " + strconv.Itoa(len(content)) + "\x00" + content
	sum := sha1.Sum([]byte(data))
	hexSum := hex.EncodeToString(sum[:])
	objDir := filepath.Join(repoDir, "objects", hexSum[:2])
	require.NoError(t, os.MkdirAll(objDir, 0o755))
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	_, err := zw.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(filepath.Join(objDir, hexSum[2:]), buf.Bytes(), 0o644))
	return hexSum
}

// trackedConn counts one live upstream connection until it is closed
type trackedConn struct {
	net.Conn
	once    sync.Once
	onClose func()
}

func (c *trackedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.onClose)
	return err
}

func (c *trackedConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return c.Conn.Close()
}
