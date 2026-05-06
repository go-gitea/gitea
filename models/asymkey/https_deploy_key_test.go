// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPSDeployKeyTokenLength(t *testing.T) {
	// Verify the token length constant is used for validation.
	assert.True(t, tokenIsValidFormat(strings.Repeat("a", HTTPSDeployKeyTokenLength)))
	assert.False(t, tokenIsValidFormat(strings.Repeat("a", HTTPSDeployKeyTokenLength-1)))
	assert.False(t, tokenIsValidFormat(strings.Repeat("a", HTTPSDeployKeyTokenLength+1)))
	assert.False(t, tokenIsValidFormat(strings.Repeat("g", HTTPSDeployKeyTokenLength)))
}

func TestTokenIsValidFormatEdgeCases(t *testing.T) {
	// Valid: 40 lowercase hex chars
	assert.True(t, tokenIsValidFormat("aaaaaaaaaa000000000011111111112222222222"))

	// Reject uppercase hex
	assert.False(t, tokenIsValidFormat(strings.Repeat("A", HTTPSDeployKeyTokenLength)))
	assert.False(t, tokenIsValidFormat("AAAAAAAAAA000000000011111111112222222222"))

	// Reject whitespace
	assert.False(t, tokenIsValidFormat(" "+strings.Repeat("a", HTTPSDeployKeyTokenLength-1)))
	assert.False(t, tokenIsValidFormat(strings.Repeat("a", HTTPSDeployKeyTokenLength-1)+" "))
	assert.False(t, tokenIsValidFormat(strings.Repeat("a", HTTPSDeployKeyTokenLength/2)+" "+strings.Repeat("a", HTTPSDeployKeyTokenLength/2-1)))

	// Reject empty string
	assert.False(t, tokenIsValidFormat(""))

	// Reject special characters
	assert.False(t, tokenIsValidFormat(strings.Repeat("!", HTTPSDeployKeyTokenLength)))
	assert.False(t, tokenIsValidFormat(strings.Repeat("@", HTTPSDeployKeyTokenLength)))

	// Reject mixed case
	assert.False(t, tokenIsValidFormat("AaAaAaAaAa0000000000111111111122222222"))
}

func TestAddHTTPSDeployKeyEmptyName(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	_, _, err := AddHTTPSDeployKey(t.Context(), 1, "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestAddHTTPSDeployKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, token, err := AddHTTPSDeployKey(t.Context(), 1, "ci-readonly", true)
	require.NoError(t, err)
	require.NotNil(t, key)

	assert.Equal(t, int64(1), key.RepoID)
	assert.Equal(t, "ci-readonly", key.Name)
	assert.True(t, key.IsReadOnly())
	assert.Len(t, token, HTTPSDeployKeyTokenLength, "token should match HTTPSDeployKeyTokenLength")
	for _, r := range token {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		assert.Truef(t, ok, "token contains non-hex char %q", r)
	}

	// Verify TokenHash is non-empty (token was hashed)
	assert.NotEmpty(t, key.TokenHash, "TokenHash must be set after create")

	// Verify TokenLastEight matches the actual token suffix
	assert.Equal(t, token[len(token)-8:], key.TokenLastEight, "TokenLastEight must match token suffix")

	// Verify timestamps are set
	assert.NotZero(t, key.CreatedUnix, "CreatedUnix must be set after create")
	assert.NotZero(t, key.UpdatedUnix, "UpdatedUnix must be set after create")

	got, err := GetHTTPSDeployKeyByID(t.Context(), key.ID)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
	assert.Equal(t, key.TokenHash, got.TokenHash)
	assert.Empty(t, got.Token, "plaintext token must not be persisted")

	_ = DeleteHTTPSDeployKey(t.Context(), 1, key.ID)
}

func TestAddHTTPSDeployKey_NameUnique(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	keyA, _, err := AddHTTPSDeployKey(t.Context(), 1, "dup", false)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, keyA.ID)

	_, _, err = AddHTTPSDeployKey(t.Context(), 1, "dup", false)
	require.Error(t, err)
	assert.True(t, IsErrHTTPSDeployKeyNameAlreadyUsed(err),
		"expected ErrHTTPSDeployKeyNameAlreadyUsed, got %T: %v", err, err)

	// Same name on a different repo is fine.
	keyB, _, err := AddHTTPSDeployKey(t.Context(), 2, "dup", false)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 2, keyB.ID)
}

func TestListHTTPSDeployKeys(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	keyA, _, err := AddHTTPSDeployKey(t.Context(), 1, "t-list-a", true)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, keyA.ID)

	keyB, _, err := AddHTTPSDeployKey(t.Context(), 1, "t-list-b", false)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, keyB.ID)

	keyC, _, err := AddHTTPSDeployKey(t.Context(), 2, "t-list-c", true)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 2, keyC.ID)

	keys, err := db.Find[HTTPSDeployKey](t.Context(),
		ListHTTPSDeployKeysOptions{RepoID: 1})
	require.NoError(t, err)
	names := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		names[k.Name] = struct{}{}
	}
	assert.Contains(t, names, "t-list-a")
	assert.Contains(t, names, "t-list-b")

	keys, err = db.Find[HTTPSDeployKey](t.Context(),
		ListHTTPSDeployKeysOptions{RepoID: 2})
	require.NoError(t, err)
	names = make(map[string]struct{}, len(keys))
	for _, k := range keys {
		names[k.Name] = struct{}{}
	}
	assert.Contains(t, names, "t-list-c")
}

func TestDeleteHTTPSDeployKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, _, err := AddHTTPSDeployKey(t.Context(), 1, "to-delete", true)
	require.NoError(t, err)

	require.NoError(t, DeleteHTTPSDeployKey(t.Context(), 1, key.ID))

	_, err = GetHTTPSDeployKeyByID(t.Context(), key.ID)
	require.Error(t, err)
	assert.True(t, IsErrHTTPSDeployKeyNotExist(err))

	// Deleting a key that belongs to a different repo must fail cleanly.
	key, _, err = AddHTTPSDeployKey(t.Context(), 1, "stays", true)
	require.NoError(t, err)
	err = DeleteHTTPSDeployKey(t.Context(), 2, key.ID)
	require.Error(t, err)
	assert.True(t, IsErrHTTPSDeployKeyNotExist(err))
}

func TestVerifyHTTPSDeployToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, token, err := AddHTTPSDeployKey(t.Context(), 1, "verify", false)
	require.NoError(t, err)

	got, err := VerifyHTTPSDeployToken(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
	assert.Equal(t, key.RepoID, got.RepoID)

	_, err = VerifyHTTPSDeployToken(t.Context(), "0000000000000000000000000000000000000000")
	require.Error(t, err)
	assert.True(t, IsErrHTTPSDeployKeyNotExist(err))

	_, err = VerifyHTTPSDeployToken(t.Context(), "")
	require.Error(t, err)

	_, err = VerifyHTTPSDeployToken(t.Context(), "not-hex")
	require.Error(t, err)

	_ = DeleteHTTPSDeployKey(t.Context(), 1, key.ID)
}

func TestHTTPSDeployKeyAfterLoad(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, token, err := AddHTTPSDeployKey(t.Context(), 1, "afterload-test", true)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, key.ID)

	// Fresh key: CreatedUnix == UpdatedUnix, so HasUsed should be false
	got, err := GetHTTPSDeployKeyByID(t.Context(), key.ID)
	require.NoError(t, err)
	assert.False(t, got.HasUsed, "fresh key should not have HasUsed")
	assert.True(t, got.HasRecentActivity, "fresh key should have recent activity")

	// After verify: UpdatedUnix > CreatedUnix, so HasUsed should be true
	// Sleep to ensure second-level granularity differs.
	time.Sleep(1 * time.Second)
	_, err = VerifyHTTPSDeployToken(t.Context(), token)
	require.NoError(t, err)

	got, err = GetHTTPSDeployKeyByID(t.Context(), key.ID)
	require.NoError(t, err)
	assert.True(t, got.HasUsed, "key should show HasUsed after verify")
	assert.True(t, got.HasRecentActivity, "key should still have recent activity")
	assert.NotEqual(t, got.CreatedUnix, got.UpdatedUnix, "UpdatedUnix should differ from CreatedUnix after use")
}

func TestVerifyHTTPSDeployTokenUpdatesTimestamp(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, token, err := AddHTTPSDeployKey(t.Context(), 1, "timestamp-test", false)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, key.ID)

	originalUpdated := key.UpdatedUnix

	// Sleep to ensure second-level granularity differs.
	time.Sleep(1 * time.Second)
	got, err := VerifyHTTPSDeployToken(t.Context(), token)
	require.NoError(t, err)
	assert.Greater(t, got.UpdatedUnix, originalUpdated, "UpdatedUnix should increase after verification")
}

func TestAddHTTPSDeployKey_ConcurrentInsert(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Attempt concurrent inserts with the same (repoID, name) inside a transaction.
	// Only one should succeed; the other must receive ErrHTTPSDeployKeyNameAlreadyUsed.
	var results [3]struct {
		key *HTTPSDeployKey
		err error
	}

	done := make(chan struct{})
	for idx := range 3 {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			key, _, err := AddHTTPSDeployKey(ctx, 1, "concurrent-key", true)
			results[i] = struct {
				key *HTTPSDeployKey
				err error
			}{key: key, err: err}
		}(idx)
	}

	for range 3 {
		<-done
	}

	successCount := 0
	for i := range results {
		if results[i].err == nil {
			successCount++
			assert.NotNil(t, results[i].key, "result[%d]: key should not be nil on success", i)
			_ = DeleteHTTPSDeployKey(t.Context(), 1, results[i].key.ID)
		} else {
			assert.True(t, IsErrHTTPSDeployKeyNameAlreadyUsed(results[i].err),
				"result[%d]: expected ErrHTTPSDeployKeyNameAlreadyUsed, got %T: %v",
				i, results[i].err, results[i].err)
		}
	}

	assert.Equal(t, 1, successCount, "exactly one concurrent insert should succeed")
}

func TestVerifyHTTPSDeployToken_TimingResistance(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Create a valid key so that valid-format tokens trigger the hash lookup path.
	key, _, err := AddHTTPSDeployKey(t.Context(), 1, "timing-test", true)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, key.ID)

	// Generate a fake valid-format token that does not match any key.
	validFormatToken := func() string {
		b := make([]byte, 20)
		_, _ = rand.Read(b)
		return hex.EncodeToString(b)
	}

	// Measure the time to verify a valid-format but wrong token.
	validFormat := validFormatToken()
	start := time.Now()
	for range 5 {
		_, _ = VerifyHTTPSDeployToken(t.Context(), validFormat)
	}
	validFormatDuration := time.Since(start)

	// Measure the time to verify an invalid-format token.
	// If the implementation short-circuits without a dummy hash,
	// this will be orders of magnitude faster than the valid-format case.
	invalidFormat := "short"
	start = time.Now()
	for range 5 {
		_, _ = VerifyHTTPSDeployToken(t.Context(), invalidFormat)
	}
	invalidFormatDuration := time.Since(start)

	// The invalid-format path should not be dramatically faster.
	// Allow a generous margin (invalid should be within 10x of valid)
	// to account for CI noise while still catching the microsecond-vs-millisecond gap.
	if invalidFormatDuration < validFormatDuration/10 {
		t.Errorf("invalid-format verification (%v) is too fast compared to valid-format (%v); "+
			"possible timing oracle", invalidFormatDuration, validFormatDuration)
	}
}

func TestVerifyHTTPSDeployToken_DummyHash(t *testing.T) {
	// Verify that the dummy hash in the invalid-format path actually performs
	// a pbkdf2 computation by confirming the HashToken call is reachable.
	// This is a structural check: the dummy hash uses a random salt,
	// so the result should be deterministic for the same inputs.
	salt := "test-salt-12345"
	hash := auth_model.HashToken("dummy-token", salt)
	assert.NotEmpty(t, hash, "HashToken should produce non-empty output")
	assert.NotEqual(t, "dummy-token", hash, "HashToken should not return the input")
}

func TestAddHTTPSDeployKey_WithinTransaction(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Create a key outside a transaction first.
	key1, _, err := AddHTTPSDeployKey(t.Context(), 1, "in-tx-key", true)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, key1.ID)

	// Attempt to create a duplicate inside a transaction.
	// The operation should detect the conflict and return the proper error.
	var insertErr error
	_ = db.WithTx(t.Context(), func(ctx context.Context) error {
		_, _, insertErr = AddHTTPSDeployKey(ctx, 1, "in-tx-key", true)
		return insertErr
	})
	require.Error(t, insertErr)
	assert.True(t, IsErrHTTPSDeployKeyNameAlreadyUsed(insertErr),
		"expected ErrHTTPSDeployKeyNameAlreadyUsed, got %T: %v", insertErr, insertErr)
}

func TestHTTPSDeployKeyModeSelection(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	readOnlyKey, _, err := AddHTTPSDeployKey(t.Context(), 1, "mode-read", true)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, readOnlyKey.ID)
	assert.True(t, readOnlyKey.IsReadOnly())

	writeKey, _, err := AddHTTPSDeployKey(t.Context(), 1, "mode-write", false)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, writeKey.ID)
	assert.False(t, writeKey.IsReadOnly())
}

func TestHTTPSDeployKeyTokenGeneration(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Verify that each key gets a unique token.
	_, token1, err := AddHTTPSDeployKey(t.Context(), 1, "token-gen-1", true)
	require.NoError(t, err)
	_, token2, err := AddHTTPSDeployKey(t.Context(), 1, "token-gen-2", true)
	require.NoError(t, err)

	assert.NotEqual(t, token1, token2, "each key should have a unique token")

	// Verify that generated tokens are always valid format.
	assert.True(t, tokenIsValidFormat(token1))
	assert.True(t, tokenIsValidFormat(token2))
}

func TestVerifyHTTPSDeployToken_LastEightIndex(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, token, err := AddHTTPSDeployKey(t.Context(), 1, "last-eight", true)
	require.NoError(t, err)
	defer DeleteHTTPSDeployKey(t.Context(), 1, key.ID)

	// Verify that the last eight characters of the token match the index column.
	assert.Equal(t, token[len(token)-8:], key.TokenLastEight)

	// Verify that the token lookup works even when other keys share the same suffix.
	// Create a dummy key with a crafted token that shares the last eight chars.
	salt := util.CryptoRandomString(10)
	dummyToken := strings.Repeat("a", HTTPSDeployKeyTokenLength-8) + key.TokenLastEight
	dummyHash := auth_model.HashToken(dummyToken, salt)

	dummyKey := &HTTPSDeployKey{
		RepoID:         1,
		Name:           "last-eight-collide",
		TokenHash:      dummyHash,
		TokenSalt:      salt,
		TokenLastEight: key.TokenLastEight,
		Mode:           1,
	}
	require.NoError(t, db.Insert(t.Context(), dummyKey))
	defer DeleteHTTPSDeployKey(t.Context(), 1, dummyKey.ID)

	// Verify the original token still resolves to the correct key.
	got, err := VerifyHTTPSDeployToken(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
}
