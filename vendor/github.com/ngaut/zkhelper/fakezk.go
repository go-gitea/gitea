// Copyright 2013, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fakezk is a pretty complete mock implementation of a
// Zookeper connection (see go/zk/zk.Conn). All operations
// work as expected with the exceptions of zk.Conn.ACL and
// zk.Conn.SetACL. zk.Conn.SetACL will succeed, but it is a noop (and
// the ACLs won't be respected). zk.Conn.ACL will panic. It is OK to
// access the connection from multiple goroutines, but the locking is
// very naive (every operation locks the whole connection).
package zkhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/ngaut/go-zookeeper/zk"
)

type zconn struct {
	mu           sync.Mutex
	root         *stat
	zxid         int64
	existWatches map[string][]chan zk.Event
}

func (conn *zconn) getZxid() int64 {
	conn.zxid++
	return conn.zxid
}

func (conn *zconn) Seq2Str(seq int64) string {
	return fmt.Sprintf("%0.10d", seq)
}

// NewConn returns a fake zk.Conn implementation. Data is stored in
// memory, and there's a global connection lock for concurrent access.
func NewConn() Conn {
	return &zconn{
		root: &stat{
			name:     "/",
			children: make(map[string]*stat),
		},
		existWatches: make(map[string][]chan zk.Event)}
}

// NewConnFromFile returns a fake zk.Conn implementation, that is seeded
// with the json data extracted from the input file.
func NewConnFromFile(filename string) Conn {
	result := &zconn{
		root: &stat{
			name:     "/",
			children: make(map[string]*stat),
		},
		existWatches: make(map[string][]chan zk.Event)}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Errorf("NewConnFromFile failed to read file %v: %v", filename, err))
	}
	values := make(map[string]interface{})
	if err := json.Unmarshal(data, &values); err != nil {
		panic(fmt.Errorf("NewConnFromFile failed to json.Unmarshal file %v: %v", filename, err))
	}
	for k, v := range values {
		jv, err := json.Marshal(v)
		if err != nil {
			panic(fmt.Errorf("NewConnFromFile failed to json.Marshal value %v: %v", k, err))
		}

		// CreateRecursive will work for a leaf node where the parent
		// doesn't exist, but not for a node in the middle of a tree
		// that already exists. So have to use 'Set' as a backup.
		if _, err := CreateRecursive(result, k, string(jv), 0, nil); err != nil {
			if ZkErrorEqual(err, zk.ErrNodeExists) {
				_, err = result.Set(k, jv, -1)
			}
			if err != nil {
				panic(fmt.Errorf("NewConnFromFile failed to zk.CreateRecursive value %v: %v", k, err))
			}
		}
	}
	return result
}

func (conn *zconn) GetACL(path string) ([]zk.ACL, zk.Stat, error) {
	return nil, nil, nil
}

func (conn *zconn) Get(zkPath string) (data []byte, stat zk.Stat, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	node, _, rest, err := conn.getNode(zkPath, "get")
	if err != nil {
		return nil, nil, err
	}
	if len(rest) != 0 {
		return nil, nil, zkError(zk.ErrNoNode, "get", zkPath)
	}
	return []byte(node.content), node, nil
}

func (conn *zconn) GetW(zkPath string) (data []byte, stat zk.Stat, watch <-chan zk.Event, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	node, _, rest, err := conn.getNode(zkPath, "getw")
	if err != nil {
		return nil, nil, nil, err
	}

	if len(rest) != 0 {
		return nil, nil, nil, zkError(zk.ErrNoNode, "getw", zkPath)
	}
	c := make(chan zk.Event, 1)
	node.changeWatches = append(node.changeWatches, c)
	return []byte(node.content), node, c, nil
}

func (conn *zconn) Children(zkPath string) (children []string, stat zk.Stat, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	//println("Children:", conn.String())

	node, _, rest, err := conn.getNode(zkPath, "children")
	if err != nil {
		return nil, nil, err
	}

	if len(rest) != 0 {
		return nil, nil, zkError(zk.ErrNoNode, "children", zkPath)
	}
	for name := range node.children {
		children = append(children, name)
	}
	return children, node, nil
}

func (conn *zconn) ChildrenW(zkPath string) (children []string, stat zk.Stat, watch <-chan zk.Event, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	//println("ChildrenW:", conn.String())

	node, _, rest, err := conn.getNode(zkPath, "childrenw")
	if err != nil {
		return nil, nil, nil, err
	}

	if len(rest) != 0 {
		return nil, nil, nil, zkError(zk.ErrNoNode, "childrenw", zkPath)
	}
	c := make(chan zk.Event, 1)
	node.childrenWatches = append(node.childrenWatches, c)
	for name := range node.children {
		children = append(children, name)
	}
	return children, node, c, nil
}

func (conn *zconn) Exists(zkPath string) (exist bool, stat zk.Stat, err error) {
	// FIXME(szopa): if the path is bad, Op will be "get."
	exist = false
	_, stat, err = conn.Get(zkPath)
	if err != nil {
		if ZkErrorEqual(err, zk.ErrNoNode) {
			err = nil
		}
	} else {
		exist = true
	}

	return exist, stat, err
}

func (conn *zconn) ExistsW(zkPath string) (exist bool, stat zk.Stat, watch <-chan zk.Event, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	exist = false
	c := make(chan zk.Event, 1)
	node, _, rest, err := conn.getNode(zkPath, "existsw")
	if err != nil {
		return exist, nil, nil, err
	}

	if len(rest) != 0 {
		watches, ok := conn.existWatches[zkPath]
		if !ok {
			watches = make([]chan zk.Event, 0)
			conn.existWatches[zkPath] = watches
		}
		conn.existWatches[zkPath] = append(watches, c)
		return exist, nil, c, nil
	}

	exist = true
	node.existWatches = append(node.existWatches, c)
	return exist, node, c, nil
}

func (conn *zconn) Create(zkPath string, value []byte, flags int32, aclv []zk.ACL) (zkPathCreated string, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	node, _, rest, err := conn.getNode(zkPath, "create")
	if err != nil {
		return "", err
	}
	if len(rest) == 0 {
		return "", zkError(zk.ErrNodeExists, "create", zkPath)
	}

	if len(rest) > 1 {
		return "", zkError(zk.ErrNoNode, "create", zkPath)
	}

	zxid := conn.getZxid()
	name := rest[0]
	if (flags & zk.FlagSequence) != 0 {
		sequence := node.nextSequence()
		name += sequence
		zkPath = zkPath + sequence
	}

	stat := &stat{
		name:         name,
		content:      string(value),
		children:     make(map[string]*stat),
		acl:          aclv,
		mtime:        time.Now(),
		ctime:        time.Now(),
		czxid:        zxid,
		mzxid:        zxid,
		existWatches: make([]chan zk.Event, 0),
	}
	node.children[name] = stat
	event := zk.Event{
		Type:  zk.EventNodeCreated,
		Path:  zkPath,
		State: zk.StateConnected,
	}
	if watches, ok := conn.existWatches[zkPath]; ok {
		delete(conn.existWatches, zkPath)
		for _, watch := range watches {
			watch <- event

		}
	}
	childrenEvent := zk.Event{
		Type:  zk.EventNodeChildrenChanged,
		Path:  zkPath,
		State: zk.StateConnected,
	}
	for _, watch := range node.childrenWatches {
		watch <- childrenEvent
		close(watch)
	}
	node.childrenWatches = nil

	node.cversion++

	return zkPath, nil
}

func (conn *zconn) Set(zkPath string, value []byte, version int32) (stat zk.Stat, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	node, _, rest, err := conn.getNode(zkPath, "set")
	if err != nil {
		return nil, err
	}

	if len(rest) != 0 {
		return nil, zkError(zk.ErrNoNode, "set", zkPath)
	}

	if version != -1 && node.version != int(version) {
		return nil, zkError(zk.ErrBadVersion, "set", zkPath)
	}
	node.content = string(value)
	node.version++
	for _, watch := range node.changeWatches {
		watch <- zk.Event{
			Type:  zk.EventNodeDataChanged,
			Path:  zkPath,
			State: zk.StateConnected,
		}
	}
	node.changeWatches = nil
	return node, nil
}

func (conn *zconn) Delete(zkPath string, version int32) (err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	node, parent, rest, err := conn.getNode(zkPath, "delete")
	if err != nil {
		return err
	}

	if len(rest) > 0 {
		return zkError(zk.ErrNoNode, "delete", zkPath)
	}
	if len(node.children) > 0 {
		return zkError(zk.ErrNotEmpty, "delete", zkPath)
	}
	delete(parent.children, node.name)
	event := zk.Event{
		Type:  zk.EventNodeDeleted,
		Path:  zkPath,
		State: zk.StateConnected,
	}
	for _, watch := range node.existWatches {
		watch <- event
	}
	for _, watch := range node.changeWatches {
		watch <- event
	}
	node.existWatches = nil
	node.changeWatches = nil
	childrenEvent := zk.Event{
		Type:  zk.EventNodeChildrenChanged,
		Path:  zkPath,
		State: zk.StateConnected}

	for _, watch := range parent.childrenWatches {
		watch <- childrenEvent
	}
	return nil
}

func (conn *zconn) Close() {
	conn.mu.Lock()
	defer conn.mu.Unlock()
		for _, watches := range conn.existWatches {
			for _, c := range watches {
				close(c)
			}
		}
		conn.root.closeAllWatches()
}

/*
func (conn *zconn) RetryChange(path string, flags int, acl []zk.ACL, changeFunc zk.ChangeFunc) error {
	for {
		oldValue, oldStat, err := conn.Get(path)
		if err != nil && !ZkErrorEqual(err, zk.ErrNoNode) {
			return err
		}
		newValue, err := changeFunc(oldValue, oldStat)
		if err != nil {
			return err
		}
		if oldStat == nil {
			_, err := conn.Create(path, newValue, flags, acl)
			if err == nil || !ZkErrorEqual(err, zk.ZNODEEXISTS) {
				return err
			}
			continue
		}
		if newValue == oldValue {
			return nil // Nothing to do.
		}
		_, err = conn.Set(path, newValue, oldStat.Version())
		if err == nil || !ZkErrorEqual(err, zk.ZBADVERSION) && !ZkErrorEqual(err, zk.ErrNoNode) {
			return err
		}
	}
}
*/

func (conn *zconn) SetACL(zkPath string, aclv []zk.ACL, version int32) (zk.Stat, error) {
	return nil, nil
}

func (conn *zconn) getNode(zkPath string, op string) (node *stat, parent *stat, rest []string, err error) {
	// FIXME(szopa): Make sure the path starts with /.
	parts := strings.Split(zkPath, "/")
	if parts[0] != "" {
		//todo: fix this, error bad arguments
		return nil, nil, nil, zkError(zk.ErrUnknown, op, zkPath)
	}
	elements := parts[1:]
	parent = nil
	current := conn.root
	for i, el := range elements {
		candidateParent := current
		candidate, ok := current.children[el]
		if !ok {
			return current, parent, elements[i:], nil
		}
		current, parent = candidate, candidateParent
	}
	return current, parent, []string{}, nil
}

type ZkError struct {
	Code error
	Op   string
	Path string
}

func (ze *ZkError) Error() string {
	return ze.Code.Error()
}

// zkError creates an appropriate error return from
// a ZooKeeper status
func zkError(code error, op, path string) error {
	return &ZkError{
		Op:   op,
		Code: code,
		Path: path,
	}
}

type stat struct {
	name     string
	content  string
	children map[string]*stat
	acl      []zk.ACL
	mtime    time.Time
	ctime    time.Time
	czxid    int64
	mzxid    int64
	pzxid    int64
	version  int
	cversion int
	aversion int

	sequence int

	existWatches    []chan zk.Event
	changeWatches   []chan zk.Event
	childrenWatches []chan zk.Event
}

func (st stat) closeAllWatches() {
	for _, c := range st.existWatches {
		close(c)
	}
	for _, c := range st.changeWatches {
		close(c)
	}
	for _, c := range st.childrenWatches {
		close(c)
	}
	for _, child := range st.children {
		child.closeAllWatches()
	}
}

func (st stat) Czxid() int64 {
	return st.czxid
}
func (st stat) Mzxid() int64 {
	return st.mzxid
}
func (st stat) CTime() time.Time {
	return st.ctime
}
func (st stat) MTime() time.Time {
	return st.mtime
}
func (st stat) Version() int {
	return st.version
}
func (st stat) CVersion() int {
	return st.cversion
}
func (st stat) AVersion() int {
	return st.aversion
}
func (st stat) EphemeralOwner() int64 {
	return 0
}

func (st stat) DataLength() int {
	return len(st.content)
}

func (st stat) NumChildren() int {
	return len(st.children)
}

func (st stat) Pzxid() int64 {
	return st.pzxid
}

func (st *stat) nextSequence() string {
	st.sequence++
	return fmt.Sprintf("%010d", st.sequence)
}

func (st stat) fprintRecursive(level int, buf *bytes.Buffer) {
	start := strings.Repeat("  ", level)
	fmt.Fprintf(buf, "%v-%v:\n", start, st.name)
	if st.content != "" {
		fmt.Fprintf(buf, "%v content: %q\n\n", start, st.content)
	}
	if len(st.children) > 0 {
		for _, child := range st.children {
			child.fprintRecursive(level+1, buf)
		}
	}
}

func (conn *zconn) String() string {
	b := new(bytes.Buffer)
	conn.root.fprintRecursive(0, b)
	return b.String()
}
