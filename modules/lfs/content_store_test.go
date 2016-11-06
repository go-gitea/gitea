package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

var contentStore *ContentStore

func TestContenStorePut(t *testing.T) {
	setup()
	defer teardown()

	m := &MetaObject{
		Oid:  "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
		Size: 12,
	}

	b := bytes.NewBuffer([]byte("test content"))

	if err := contentStore.Put(m, b); err != nil {
		t.Fatalf("expected put to succeed, got: %s", err)
	}

	path := "content-store-test/6a/e8/a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected content to exist after putting")
	}
}

func TestContenStorePutHashMismatch(t *testing.T) {
	setup()
	defer teardown()

	m := &MetaObject{
		Oid:  "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
		Size: 12,
	}

	b := bytes.NewBuffer([]byte("bogus content"))

	if err := contentStore.Put(m, b); err == nil {
		t.Fatal("expected put with bogus content to fail")
	}

	path := "content-store-test/6a/e8/a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected content to not exist after putting bogus content")
	}
}

func TestContenStorePutSizeMismatch(t *testing.T) {
	setup()
	defer teardown()

	m := &MetaObject{
		Oid:  "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
		Size: 14,
	}

	b := bytes.NewBuffer([]byte("test content"))

	if err := contentStore.Put(m, b); err == nil {
		t.Fatal("expected put with bogus size to fail")
	}

	path := "content-store-test/6a/e8/a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected content to not exist after putting bogus size")
	}
}

func TestContenStoreGet(t *testing.T) {
	setup()
	defer teardown()

	m := &MetaObject{
		Oid:  "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
		Size: 12,
	}

	b := bytes.NewBuffer([]byte("test content"))

	if err := contentStore.Put(m, b); err != nil {
		t.Fatalf("expected put to succeed, got: %s", err)
	}

	r, err := contentStore.Get(m, 0)
	if err != nil {
		t.Fatalf("expected get to succeed, got: %s", err)
	}

	by, _ := ioutil.ReadAll(r)
	if string(by) != "test content" {
		t.Fatalf("expected to read content, got: %s", string(by))
	}
}

func TestContenStoreGetWithRange(t *testing.T) {
	setup()
	defer teardown()

	m := &MetaObject{
		Oid:  "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
		Size: 12,
	}

	b := bytes.NewBuffer([]byte("test content"))

	if err := contentStore.Put(m, b); err != nil {
		t.Fatalf("expected put to succeed, got: %s", err)
	}

	r, err := contentStore.Get(m, 5)
	if err != nil {
		t.Fatalf("expected get to succeed, got: %s", err)
	}

	by, _ := ioutil.ReadAll(r)
	if string(by) != "content" {
		t.Fatalf("expected to read content, got: %s", string(by))
	}
}

func TestContenStoreGetNonExisting(t *testing.T) {
	setup()
	defer teardown()

	_, err := contentStore.Get(&MetaObject{Oid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, 0)
	if err == nil {
		t.Fatalf("expected to get an error, but content existed")
	}
}

func TestContenStoreExists(t *testing.T) {
	setup()
	defer teardown()

	m := &MetaObject{
		Oid:  "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
		Size: 12,
	}

	b := bytes.NewBuffer([]byte("test content"))

	if contentStore.Exists(m) {
		t.Fatalf("expected content to not exist yet")
	}

	if err := contentStore.Put(m, b); err != nil {
		t.Fatalf("expected put to succeed, got: %s", err)
	}

	if !contentStore.Exists(m) {
		t.Fatalf("expected content to exist")
	}
}

func setup() {
	store, err := NewContentStore("content-store-test")
	if err != nil {
		fmt.Printf("error initializing content store: %s\n", err)
		os.Exit(1)
	}
	contentStore = store
}

func teardown() {
	os.RemoveAll("content-store-test")
}
