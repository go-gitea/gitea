package main

import (
	"fmt"
	"os"
	"testing"
)

var (
	metaStoreTest *MetaStore
)

func TestGetWithAuth(t *testing.T) {
	setupMeta()
	defer teardownMeta()

	meta, err := metaStoreTest.Get(&RequestVars{Authorization: testAuth, Oid: contentOid})
	if err != nil {
		t.Fatalf("Error retreiving meta: %s", err)
	}

	if meta.Oid != contentOid {
		t.Errorf("expected to get content oid, got: %s", meta.Oid)
	}

	if meta.Size != contentSize {
		t.Errorf("expected to get content size, got: %d", meta.Size)
	}
}

func TestGetWithoutAuth(t *testing.T) {
	setupMeta()
	defer teardownMeta()

	_, err := metaStoreTest.Get(&RequestVars{Authorization: badAuth, Oid: contentOid})
	if !isAuthError(err) {
		t.Errorf("expected auth error, got: %s", err)
	}
}

func TestPutWithAuth(t *testing.T) {
	setupMeta()
	defer teardownMeta()

	meta, err := metaStoreTest.Put(&RequestVars{Authorization: testAuth, Oid: nonexistingOid, Size: 42})
	if err != nil {
		t.Errorf("expected put to succeed, got : %s", err)
	}

	if meta.Existing {
		t.Errorf("expected meta to not have existed")
	}

	meta, err = metaStoreTest.Get(&RequestVars{Authorization: testAuth, Oid: nonexistingOid})
	if err != nil {
		t.Errorf("expected to be able to retreive new put, got : %s", err)
	}

	if meta.Oid != nonexistingOid {
		t.Errorf("expected oids to match, got: %s", meta.Oid)
	}

	if meta.Size != 42 {
		t.Errorf("expected sizes to match, got: %d", meta.Size)
	}

	meta, err = metaStoreTest.Put(&RequestVars{Authorization: testAuth, Oid: nonexistingOid, Size: 42})
	if err != nil {
		t.Errorf("expected put to succeed, got : %s", err)
	}

	if !meta.Existing {
		t.Errorf("expected meta to now exist")
	}
}

func TestPuthWithoutAuth(t *testing.T) {
	setupMeta()
	defer teardownMeta()

	_, err := metaStoreTest.Put(&RequestVars{Authorization: badAuth, Oid: contentOid, Size: 42})
	if !isAuthError(err) {
		t.Errorf("expected auth error, got: %s", err)
	}
}

func setupMeta() {
	store, err := NewMetaStore("test-meta-store.db")
	if err != nil {
		fmt.Printf("error initializing test meta store: %s\n", err)
		os.Exit(1)
	}

	metaStoreTest = store
	if err := metaStoreTest.AddUser(testUser, testPass); err != nil {
		teardownMeta()
		fmt.Printf("error adding test user to meta store: %s\n", err)
		os.Exit(1)
	}

	rv := &RequestVars{Authorization: testAuth, Oid: contentOid, Size: contentSize}
	if _, err := metaStoreTest.Put(rv); err != nil {
		teardownMeta()
		fmt.Printf("error seeding test meta store: %s\n", err)
		os.Exit(1)
	}
}

func teardownMeta() {
	os.RemoveAll("test-meta-store.db")
}
