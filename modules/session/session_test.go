// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type handleFunc func(resp http.ResponseWriter, req *http.Request, sess Store)

type failingBackend struct {
	backend
	failDestroy bool
}

func (b *failingBackend) destroy(sid string) error {
	if b.failDestroy {
		return errors.New("destroy failed")
	}
	return b.backend.destroy(sid)
}

func TestSession(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.SessionConfig.CookiePath, "/sub")()
	defer test.MockVariableValue(&setting.SessionConfig.Secure, true)()

	_, err := newBackend("mysql", "", 3600)
	assert.ErrorContains(t, err, `use "db" or "redis"`)

	t.Run("GCStopsOnShutdown", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		runGC(ctx, newMemoryBackend(3600), time.Hour)
	})

	cases := []struct {
		name         string
		newBackend   func(t *testing.T) backend
		expire       func(t *testing.T, sid string)
		assertStored func(t *testing.T, backend backend, sid string)
	}{
		{
			name:       "memory",
			newBackend: func(*testing.T) backend { return newMemoryBackend(3600) },
		},
		{
			name:       "file",
			newBackend: func(t *testing.T) backend { return newFileBackend(t.TempDir(), 3600) },
		},
		{
			name:       "db",
			newBackend: func(*testing.T) backend { return &dbBackend{maxLifetime: 3600} },
			expire: func(t *testing.T, sid string) {
				_, err := db.GetEngine(t.Context()).ID(sid).Cols("expiry").Update(&auth_model.Session{Expiry: 1})
				require.NoError(t, err)
			},
		},
		{
			name: "redis",
			newBackend: func(t *testing.T) backend {
				backend, err := newRedisBackend(test.PrepareTestRedis(t)+"?prefix=gitea-test-session-", 3600)
				require.NoError(t, err)
				return backend
			},
			assertStored: func(t *testing.T, backend backend, sid string) {
				redisStore, ok := backend.(*redisBackend)
				require.True(t, ok)
				require.NoError(t, redisStore.client.Expire(t.Context(), "gitea-test-session-"+sid, time.Minute).Err())
				_, err := backend.load(sid)
				require.NoError(t, err)
				ttl, err := redisStore.client.TTL(t.Context(), "gitea-test-session-"+sid).Result()
				require.NoError(t, err)
				assert.Greater(t, ttl, time.Minute)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &failingBackend{backend: tc.newBackend(t)}
			serve := func(sid string, handle handleFunc) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				if sid != "" {
					req.AddCookie(&http.Cookie{Name: setting.SessionConfig.CookieName, Value: sid})
				}
				resp := httptest.NewRecorder()
				sessionHandler(backend, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
					handle(resp, req, GetContextSession(req))
				})).ServeHTTP(resp, req)
				return resp
			}
			create := func(t *testing.T) string {
				cookies := serve("", func(_ http.ResponseWriter, _ *http.Request, sess Store) {
					require.NoError(t, sess.Set("key", "value"))
				}).Result().Cookies()
				require.Len(t, cookies, 1)
				return cookies[0].Value
			}
			get := func(sid string, key any) (value any) {
				serve(sid, func(_ http.ResponseWriter, _ *http.Request, sess Store) { value = sess.Get(key) })
				return value
			}

			t.Run("CookieOnlyOnceSessionHoldsData", func(t *testing.T) {
				assert.Empty(t, serve("", func(http.ResponseWriter, *http.Request, Store) {}).Result().Cookies())

				resp := serve("", func(resp http.ResponseWriter, req *http.Request, sess Store) {
					require.NoError(t, sess.Set("key", "value"))
					require.NoError(t, sess.Set("other", 1))
					http.Redirect(resp, req, "https://example.com/", http.StatusSeeOther)
				})
				cookies := resp.Result().Cookies()
				require.Len(t, cookies, 1)
				sid := cookies[0].Value
				assert.True(t, isValidSessionID(sid))
				assert.Equal(t, "/sub", cookies[0].Path)
				assert.True(t, cookies[0].HttpOnly)
				assert.True(t, cookies[0].Secure)
				assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
				if tc.assertStored != nil {
					tc.assertStored(t, backend.backend, sid)
				}

				resp = serve(sid, func(_ http.ResponseWriter, _ *http.Request, sess Store) {
					assert.Equal(t, sid, sess.ID())
					assert.Equal(t, "value", sess.Get("key"))
					require.NoError(t, sess.Set("key", "changed"))
				})
				assert.Empty(t, resp.Result().Cookies())

				resp = serve("../../etc/passwd", func(_ http.ResponseWriter, _ *http.Request, sess Store) {
					assert.True(t, isValidSessionID(sess.ID()))
					require.NoError(t, sess.Set("key", "value"))
				})
				cookies = resp.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.True(t, isValidSessionID(cookies[0].Value))
			})

			t.Run("RegenerateMovesDataToNewID", func(t *testing.T) {
				oldSID := create(t)
				var newSID string
				resp := serve(oldSID, func(resp http.ResponseWriter, req *http.Request, sess Store) {
					sess.Regenerate(resp, req)
					newSID = sess.ID()
				})
				cookies := resp.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, newSID, cookies[0].Value)
				assert.NotEqual(t, oldSID, newSID)
				assert.Nil(t, get(oldSID, "key"))
				assert.Equal(t, "value", get(newSID, "key"))

				resp = serve("malformed", func(resp http.ResponseWriter, req *http.Request, sess Store) { sess.Regenerate(resp, req) })
				assert.Empty(t, resp.Result().Cookies())
			})

			t.Run("DestroyIsNotUndoneByRelease", func(t *testing.T) {
				sid := create(t)
				var resp *httptest.ResponseRecorder
				serve(sid, func(_ http.ResponseWriter, _ *http.Request, concurrent Store) {
					resp = serve(sid, func(resp http.ResponseWriter, req *http.Request, sess Store) {
						require.NoError(t, sess.Flush())
						require.NoError(t, sess.Destroy(resp, req))
					})
					require.NoError(t, concurrent.Set("key", "changed"))
				})
				cookies := resp.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, -1, cookies[0].MaxAge)
				assert.Equal(t, "/sub", cookies[0].Path)
				assert.True(t, cookies[0].Secure)
				encoded, err := backend.load(sid)
				require.NoError(t, err)
				assert.Nil(t, encoded)
			})

			t.Run("FailedDestroyIsRetriedByRelease", func(t *testing.T) {
				sid := create(t)
				serve(sid, func(resp http.ResponseWriter, req *http.Request, sess Store) {
					backend.failDestroy = true
					require.Error(t, sess.Destroy(resp, req))
					backend.failDestroy = false
					assert.Nil(t, sess.Get("key"))
				})
				encoded, err := backend.load(sid)
				require.NoError(t, err)
				assert.Nil(t, encoded)
			})

			t.Run("EmptiedSessionIsPersisted", func(t *testing.T) {
				for _, empty := range []func(Store) error{Store.Flush, func(sess Store) error { return sess.Delete("key") }} {
					sid := create(t)
					serve(sid, func(_ http.ResponseWriter, _ *http.Request, sess Store) { require.NoError(t, empty(sess)) })
					assert.Nil(t, get(sid, "key"))
				}
			})

			t.Run("UnchangedReleaseKeepsConcurrentChanges", func(t *testing.T) {
				sid := create(t)
				serve(sid, func(_ http.ResponseWriter, _ *http.Request, reader Store) {
					serve(sid, func(_ http.ResponseWriter, _ *http.Request, writer Store) {
						require.NoError(t, writer.Set("key", "changed"))
					})
					if tc.expire != nil {
						tc.expire(t, sid)
					}
					require.NoError(t, reader.Delete("missing"))
				})
				assert.Equal(t, "changed", get(sid, "key"))
			})

			t.Run("ConcurrentNewSessionWritesThrough", func(t *testing.T) {
				sid := newSessionID()
				serve(sid, func(_ http.ResponseWriter, _ *http.Request, first Store) {
					serve(sid, func(_ http.ResponseWriter, _ *http.Request, second Store) {
						require.NoError(t, second.Set("second", 2))
					})
					require.NoError(t, first.Set("first", 1))
				})
				assert.Equal(t, 1, get(sid, "first"))
			})

			t.Run("UndecodableDataReadsAsEmptySession", func(t *testing.T) {
				sid := newSessionID()
				require.NoError(t, backend.save(sid, []byte("undecodable"), true))
				resp := serve(sid, func(_ http.ResponseWriter, _ *http.Request, sess Store) { assert.Nil(t, sess.Get("key")) })
				assert.Equal(t, http.StatusOK, resp.Code)
				encoded, err := backend.load(sid)
				require.NoError(t, err)
				assert.Nil(t, encoded)
			})
		})
	}
}

func TestFileBackendWritesAtomicallyAndExpiresByModTime(t *testing.T) {
	root := t.TempDir()
	backend := newFileBackend(root, 3600)
	sid := newSessionID()
	filename := filepath.Join(root, sid[0:1], sid[1:2], sid)
	require.NoError(t, backend.save(sid, []byte("first"), true))
	require.NoError(t, backend.save(sid, []byte("second"), false))

	entries, err := os.ReadDir(filepath.Dir(filename))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, sid, entries[0].Name())
	encoded, err := backend.load(sid)
	require.NoError(t, err)
	assert.Equal(t, "second", string(encoded))

	expired := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(filename, expired, expired))
	encoded, err = backend.load(sid)
	require.NoError(t, err)
	assert.Nil(t, encoded)
	backend.gc()
	assert.NoFileExists(t, filename)
}
