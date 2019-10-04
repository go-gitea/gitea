package couchbase

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/couchbase/goutils/logging"

	"github.com/couchbase/gomemcached"        // package name is 'gomemcached'
	"github.com/couchbase/gomemcached/client" // package name is 'memcached'
)

// HTTPClient to use for REST and view operations.
var MaxIdleConnsPerHost = 256
var ClientTimeOut = 10 * time.Second
var HTTPTransport = &http.Transport{MaxIdleConnsPerHost: MaxIdleConnsPerHost}
var HTTPClient = &http.Client{Transport: HTTPTransport, Timeout: ClientTimeOut}

// PoolSize is the size of each connection pool (per host).
var PoolSize = 64

// PoolOverflow is the number of overflow connections allowed in a
// pool.
var PoolOverflow = 16

// AsynchronousCloser turns on asynchronous closing for overflow connections
var AsynchronousCloser = false

// TCP KeepAlive enabled/disabled
var TCPKeepalive = false

// Enable MutationToken
var EnableMutationToken = false

// Enable Data Type response
var EnableDataType = false

// Enable Xattr
var EnableXattr = false

// Enable Collections
var EnableCollections = false

// TCP keepalive interval in seconds. Default 30 minutes
var TCPKeepaliveInterval = 30 * 60

// Used to decide whether to skip verification of certificates when
// connecting to an ssl port.
var skipVerify = true
var certFile = ""
var keyFile = ""
var rootFile = ""

func SetSkipVerify(skip bool) {
	skipVerify = skip
}

func SetCertFile(cert string) {
	certFile = cert
}

func SetKeyFile(cert string) {
	keyFile = cert
}

func SetRootFile(cert string) {
	rootFile = cert
}

// Allow applications to speciify the Poolsize and Overflow
func SetConnectionPoolParams(size, overflow int) {

	if size > 0 {
		PoolSize = size
	}

	if overflow > 0 {
		PoolOverflow = overflow
	}
}

// Turn off overflow connections
func DisableOverflowConnections() {
	PoolOverflow = 0
}

// Toggle asynchronous overflow closer
func EnableAsynchronousCloser(closer bool) {
	AsynchronousCloser = closer
}

// Allow TCP keepalive parameters to be set by the application
func SetTcpKeepalive(enabled bool, interval int) {

	TCPKeepalive = enabled

	if interval > 0 {
		TCPKeepaliveInterval = interval
	}
}

// AuthHandler is a callback that gets the auth username and password
// for the given bucket.
type AuthHandler interface {
	GetCredentials() (string, string, string)
}

// AuthHandler is a callback that gets the auth username and password
// for the given bucket and sasl for memcached.
type AuthWithSaslHandler interface {
	AuthHandler
	GetSaslCredentials() (string, string)
}

// MultiBucketAuthHandler is kind of AuthHandler that may perform
// different auth for different buckets.
type MultiBucketAuthHandler interface {
	AuthHandler
	ForBucket(bucket string) AuthHandler
}

// HTTPAuthHandler is kind of AuthHandler that performs more general
// for outgoing http requests than is possible via simple
// GetCredentials() call (i.e. digest auth or different auth per
// different destinations).
type HTTPAuthHandler interface {
	AuthHandler
	SetCredsForRequest(req *http.Request) error
}

// RestPool represents a single pool returned from the pools REST API.
type RestPool struct {
	Name         string `json:"name"`
	StreamingURI string `json:"streamingUri"`
	URI          string `json:"uri"`
}

// Pools represents the collection of pools as returned from the REST API.
type Pools struct {
	ComponentsVersion     map[string]string `json:"componentsVersion,omitempty"`
	ImplementationVersion string            `json:"implementationVersion"`
	IsAdmin               bool              `json:"isAdminCreds"`
	UUID                  string            `json:"uuid"`
	Pools                 []RestPool        `json:"pools"`
}

// A Node is a computer in a cluster running the couchbase software.
type Node struct {
	ClusterCompatibility int                `json:"clusterCompatibility"`
	ClusterMembership    string             `json:"clusterMembership"`
	CouchAPIBase         string             `json:"couchApiBase"`
	Hostname             string             `json:"hostname"`
	InterestingStats     map[string]float64 `json:"interestingStats,omitempty"`
	MCDMemoryAllocated   float64            `json:"mcdMemoryAllocated"`
	MCDMemoryReserved    float64            `json:"mcdMemoryReserved"`
	MemoryFree           float64            `json:"memoryFree"`
	MemoryTotal          float64            `json:"memoryTotal"`
	OS                   string             `json:"os"`
	Ports                map[string]int     `json:"ports"`
	Services             []string           `json:"services"`
	Status               string             `json:"status"`
	Uptime               int                `json:"uptime,string"`
	Version              string             `json:"version"`
	ThisNode             bool               `json:"thisNode,omitempty"`
}

// A Pool of nodes and buckets.
type Pool struct {
	BucketMap map[string]*Bucket
	Nodes     []Node

	BucketURL map[string]string `json:"buckets"`

	client *Client
}

// VBucketServerMap is the a mapping of vbuckets to nodes.
type VBucketServerMap struct {
	HashAlgorithm string   `json:"hashAlgorithm"`
	NumReplicas   int      `json:"numReplicas"`
	ServerList    []string `json:"serverList"`
	VBucketMap    [][]int  `json:"vBucketMap"`
}

type DurablitySettings struct {
	Persist PersistTo
	Observe ObserveTo
}

// Bucket is the primary entry point for most data operations.
// Bucket is a locked data structure. All access to its fields should be done using read or write locking,
// as appropriate.
//
// Some access methods require locking, but rely on the caller to do so. These are appropriate
// for calls from methods that have already locked the structure. Methods like this
// take a boolean parameter "bucketLocked".
type Bucket struct {
	sync.RWMutex
	AuthType               string             `json:"authType"`
	Capabilities           []string           `json:"bucketCapabilities"`
	CapabilitiesVersion    string             `json:"bucketCapabilitiesVer"`
	Type                   string             `json:"bucketType"`
	Name                   string             `json:"name"`
	NodeLocator            string             `json:"nodeLocator"`
	Quota                  map[string]float64 `json:"quota,omitempty"`
	Replicas               int                `json:"replicaNumber"`
	Password               string             `json:"saslPassword"`
	URI                    string             `json:"uri"`
	StreamingURI           string             `json:"streamingUri"`
	LocalRandomKeyURI      string             `json:"localRandomKeyUri,omitempty"`
	UUID                   string             `json:"uuid"`
	ConflictResolutionType string             `json:"conflictResolutionType,omitempty"`
	DDocs                  struct {
		URI string `json:"uri"`
	} `json:"ddocs,omitempty"`
	BasicStats  map[string]interface{} `json:"basicStats,omitempty"`
	Controllers map[string]interface{} `json:"controllers,omitempty"`

	// These are used for JSON IO, but isn't used for processing
	// since it needs to be swapped out safely.
	VBSMJson  VBucketServerMap `json:"vBucketServerMap"`
	NodesJSON []Node           `json:"nodes"`

	pool             *Pool
	connPools        unsafe.Pointer // *[]*connectionPool
	vBucketServerMap unsafe.Pointer // *VBucketServerMap
	nodeList         unsafe.Pointer // *[]Node
	commonSufix      string
	ah               AuthHandler        // auth handler
	ds               *DurablitySettings // Durablity Settings for this bucket
	closed           bool
}

// PoolServices is all the bucket-independent services in a pool
type PoolServices struct {
	Rev          int             `json:"rev"`
	NodesExt     []NodeServices  `json:"nodesExt"`
	Capabilities json.RawMessage `json:"clusterCapabilities"`
}

// NodeServices is all the bucket-independent services running on
// a node (given by Hostname)
type NodeServices struct {
	Services map[string]int `json:"services,omitempty"`
	Hostname string         `json:"hostname"`
	ThisNode bool           `json:"thisNode"`
}

type BucketNotFoundError struct {
	bucket string
}

func (e *BucketNotFoundError) Error() string {
	return fmt.Sprint("No bucket named " + e.bucket)
}

type BucketAuth struct {
	name    string
	saslPwd string
	bucket  string
}

func newBucketAuth(name string, pass string, bucket string) *BucketAuth {
	return &BucketAuth{name: name, saslPwd: pass, bucket: bucket}
}

func (ba *BucketAuth) GetCredentials() (string, string, string) {
	return ba.name, ba.saslPwd, ba.bucket
}

// VBServerMap returns the current VBucketServerMap.
func (b *Bucket) VBServerMap() *VBucketServerMap {
	b.RLock()
	defer b.RUnlock()
	ret := (*VBucketServerMap)(b.vBucketServerMap)
	return ret
}

func (b *Bucket) GetVBmap(addrs []string) (map[string][]uint16, error) {
	vbmap := b.VBServerMap()
	servers := vbmap.ServerList
	if addrs == nil {
		addrs = vbmap.ServerList
	}

	m := make(map[string][]uint16)
	for _, addr := range addrs {
		m[addr] = make([]uint16, 0)
	}
	for vbno, idxs := range vbmap.VBucketMap {
		if len(idxs) == 0 {
			return nil, fmt.Errorf("vbmap: No KV node no for vb %d", vbno)
		} else if idxs[0] < 0 || idxs[0] >= len(servers) {
			return nil, fmt.Errorf("vbmap: Invalid KV node no %d for vb %d", idxs[0], vbno)
		}
		addr := servers[idxs[0]]
		if _, ok := m[addr]; ok {
			m[addr] = append(m[addr], uint16(vbno))
		}
	}
	return m, nil
}

// true if node is not on the bucket VBmap
func (b *Bucket) checkVBmap(node string) bool {
	vbmap := b.VBServerMap()
	servers := vbmap.ServerList

	for _, idxs := range vbmap.VBucketMap {
		if len(idxs) == 0 {
			return true
		} else if idxs[0] < 0 || idxs[0] >= len(servers) {
			return true
		}
		if servers[idxs[0]] == node {
			return false
		}
	}
	return true
}

func (b *Bucket) GetName() string {
	b.RLock()
	defer b.RUnlock()
	ret := b.Name
	return ret
}

// Nodes returns the current list of nodes servicing this bucket.
func (b *Bucket) Nodes() []Node {
	b.RLock()
	defer b.RUnlock()
	ret := *(*[]Node)(b.nodeList)
	return ret
}

// return the list of healthy nodes
func (b *Bucket) HealthyNodes() []Node {
	nodes := []Node{}

	for _, n := range b.Nodes() {
		if n.Status == "healthy" && n.CouchAPIBase != "" {
			nodes = append(nodes, n)
		}
		if n.Status != "healthy" { // log non-healthy node
			logging.Infof("Non-healthy node; node details:")
			logging.Infof("Hostname=%v, Status=%v, CouchAPIBase=%v, ThisNode=%v", n.Hostname, n.Status, n.CouchAPIBase, n.ThisNode)
		}
	}

	return nodes
}

func (b *Bucket) getConnPools(bucketLocked bool) []*connectionPool {
	if !bucketLocked {
		b.RLock()
		defer b.RUnlock()
	}
	if b.connPools != nil {
		return *(*[]*connectionPool)(b.connPools)
	} else {
		return nil
	}
}

func (b *Bucket) replaceConnPools(with []*connectionPool) {
	b.Lock()
	defer b.Unlock()

	old := b.connPools
	b.connPools = unsafe.Pointer(&with)
	if old != nil {
		for _, pool := range *(*[]*connectionPool)(old) {
			if pool != nil {
				pool.Close()
			}
		}
	}
	return
}

func (b *Bucket) getConnPool(i int) *connectionPool {

	if i < 0 {
		return nil
	}

	p := b.getConnPools(false /* not already locked */)
	if len(p) > i {
		return p[i]
	}

	return nil
}

func (b *Bucket) getConnPoolByHost(host string, bucketLocked bool) *connectionPool {
	pools := b.getConnPools(bucketLocked)
	for _, p := range pools {
		if p != nil && p.host == host {
			return p
		}
	}

	return nil
}

// Given a vbucket number, returns a memcached connection to it.
// The connection must be returned to its pool after use.
func (b *Bucket) getConnectionToVBucket(vb uint32) (*memcached.Client, *connectionPool, error) {
	for {
		vbm := b.VBServerMap()
		if len(vbm.VBucketMap) < int(vb) {
			return nil, nil, fmt.Errorf("go-couchbase: vbmap smaller than vbucket list: %v vs. %v",
				vb, vbm.VBucketMap)
		}
		masterId := vbm.VBucketMap[vb][0]
		if masterId < 0 {
			return nil, nil, fmt.Errorf("go-couchbase: No master for vbucket %d", vb)
		}
		pool := b.getConnPool(masterId)
		conn, err := pool.Get()
		if err != errClosedPool {
			return conn, pool, err
		}
		// If conn pool was closed, because another goroutine refreshed the vbucket map, retry...
	}
}

// To get random documents, we need to cover all the nodes, so select
// a connection at random.

func (b *Bucket) getRandomConnection() (*memcached.Client, *connectionPool, error) {
	for {
		var currentPool = 0
		pools := b.getConnPools(false /* not already locked */)
		if len(pools) == 0 {
			return nil, nil, fmt.Errorf("No connection pool found")
		} else if len(pools) > 1 { // choose a random connection
			currentPool = rand.Intn(len(pools))
		} // if only one pool, currentPool defaults to 0, i.e., the only pool

		// get the pool
		pool := pools[currentPool]
		conn, err := pool.Get()
		if err != errClosedPool {
			return conn, pool, err
		}

		// If conn pool was closed, because another goroutine refreshed the vbucket map, retry...
	}
}

//
// Get a random document from a bucket. Since the bucket may be distributed
// across nodes, we must first select a random connection, and then use the
// Client.GetRandomDoc() call to get a random document from that node.
//

func (b *Bucket) GetRandomDoc() (*gomemcached.MCResponse, error) {
	// get a connection from the pool
	conn, pool, err := b.getRandomConnection()

	if err != nil {
		return nil, err
	}

	// We may need to select the bucket before GetRandomDoc()
	// will work. This is sometimes done at startup (see defaultMkConn())
	// but not always, depending on the auth type.
	_, err = conn.SelectBucket(b.Name)
	if err != nil {
		return nil, err
	}

	// get a randomm document from the connection
	doc, err := conn.GetRandomDoc()
	// need to return the connection to the pool
	pool.Return(conn)
	return doc, err
}

func (b *Bucket) getMasterNode(i int) string {
	p := b.getConnPools(false /* not already locked */)
	if len(p) > i {
		return p[i].host
	}
	return ""
}

func (b *Bucket) authHandler(bucketLocked bool) (ah AuthHandler) {
	if !bucketLocked {
		b.RLock()
		defer b.RUnlock()
	}
	pool := b.pool
	name := b.Name

	if pool != nil {
		ah = pool.client.ah
	}
	if mbah, ok := ah.(MultiBucketAuthHandler); ok {
		return mbah.ForBucket(name)
	}
	if ah == nil {
		ah = &basicAuth{name, ""}
	}
	return
}

// NodeAddresses gets the (sorted) list of memcached node addresses
// (hostname:port).
func (b *Bucket) NodeAddresses() []string {
	vsm := b.VBServerMap()
	rv := make([]string, len(vsm.ServerList))
	copy(rv, vsm.ServerList)
	sort.Strings(rv)
	return rv
}

// CommonAddressSuffix finds the longest common suffix of all
// host:port strings in the node list.
func (b *Bucket) CommonAddressSuffix() string {
	input := []string{}
	for _, n := range b.Nodes() {
		input = append(input, n.Hostname)
	}
	return FindCommonSuffix(input)
}

// A Client is the starting point for all services across all buckets
// in a Couchbase cluster.
type Client struct {
	BaseURL   *url.URL
	ah        AuthHandler
	Info      Pools
	tlsConfig *tls.Config
}

func maybeAddAuth(req *http.Request, ah AuthHandler) error {
	if hah, ok := ah.(HTTPAuthHandler); ok {
		return hah.SetCredsForRequest(req)
	}
	if ah != nil {
		user, pass, _ := ah.GetCredentials()
		req.Header.Set("Authorization", "Basic "+
			base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
	}
	return nil
}

// arbitary number, may need to be tuned #FIXME
const HTTP_MAX_RETRY = 5

// Someday golang network packages will implement standard
// error codes. Until then #sigh
func isHttpConnError(err error) bool {

	estr := err.Error()
	return strings.Contains(estr, "broken pipe") ||
		strings.Contains(estr, "broken connection") ||
		strings.Contains(estr, "connection reset")
}

var client *http.Client

func ClientConfigForX509(certFile, keyFile, rootFile string) (*tls.Config, error) {
	cfg := &tls.Config{}

	if certFile != "" && keyFile != "" {
		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{tlsCert}
	} else {
		//error need to pass both certfile and keyfile
		return nil, fmt.Errorf("N1QL: Need to pass both certfile and keyfile")
	}

	var caCert []byte
	var err1 error

	caCertPool := x509.NewCertPool()
	if rootFile != "" {
		// Read that value in
		caCert, err1 = ioutil.ReadFile(rootFile)
		if err1 != nil {
			return nil, fmt.Errorf(" Error in reading cacert file, err: %v", err1)
		}
		caCertPool.AppendCertsFromPEM(caCert)
	}

	cfg.RootCAs = caCertPool
	return cfg, nil
}

func doHTTPRequest(req *http.Request) (*http.Response, error) {

	var err error
	var res *http.Response

	// we need a client that ignores certificate errors, since we self-sign
	// our certs
	if client == nil && req.URL.Scheme == "https" {
		var tr *http.Transport

		if skipVerify {
			tr = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		} else {
			// Handle cases with cert

			cfg, err := ClientConfigForX509(certFile, keyFile, rootFile)
			if err != nil {
				return nil, err
			}

			tr = &http.Transport{
				TLSClientConfig: cfg,
			}
		}

		client = &http.Client{Transport: tr}

	} else if client == nil {
		client = HTTPClient
	}

	for i := 0; i < HTTP_MAX_RETRY; i++ {
		res, err = client.Do(req)
		if err != nil && isHttpConnError(err) {
			continue
		}
		break
	}

	if err != nil {
		return nil, err
	}

	return res, err
}

func doPutAPI(baseURL *url.URL, path string, params map[string]interface{}, authHandler AuthHandler, out interface{}) error {
	return doOutputAPI("PUT", baseURL, path, params, authHandler, out)
}

func doPostAPI(baseURL *url.URL, path string, params map[string]interface{}, authHandler AuthHandler, out interface{}) error {
	return doOutputAPI("POST", baseURL, path, params, authHandler, out)
}

func doOutputAPI(
	httpVerb string,
	baseURL *url.URL,
	path string,
	params map[string]interface{},
	authHandler AuthHandler,
	out interface{}) error {

	var requestUrl string

	if q := strings.Index(path, "?"); q > 0 {
		requestUrl = baseURL.Scheme + "://" + baseURL.Host + path[:q] + "?" + path[q+1:]
	} else {
		requestUrl = baseURL.Scheme + "://" + baseURL.Host + path
	}

	postData := url.Values{}
	for k, v := range params {
		postData.Set(k, fmt.Sprintf("%v", v))
	}

	req, err := http.NewRequest(httpVerb, requestUrl, bytes.NewBufferString(postData.Encode()))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	err = maybeAddAuth(req, authHandler)
	if err != nil {
		return err
	}

	res, err := doHTTPRequest(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != 200 {
		bod, _ := ioutil.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("HTTP error %v getting %q: %s",
			res.Status, requestUrl, bod)
	}

	d := json.NewDecoder(res.Body)
	if err = d.Decode(&out); err != nil {
		return err
	}
	return nil
}

func queryRestAPI(
	baseURL *url.URL,
	path string,
	authHandler AuthHandler,
	out interface{}) error {

	var requestUrl string

	if q := strings.Index(path, "?"); q > 0 {
		requestUrl = baseURL.Scheme + "://" + baseURL.Host + path[:q] + "?" + path[q+1:]
	} else {
		requestUrl = baseURL.Scheme + "://" + baseURL.Host + path
	}

	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return err
	}

	err = maybeAddAuth(req, authHandler)
	if err != nil {
		return err
	}

	res, err := doHTTPRequest(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != 200 {
		bod, _ := ioutil.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("HTTP error %v getting %q: %s",
			res.Status, requestUrl, bod)
	}

	d := json.NewDecoder(res.Body)
	if err = d.Decode(&out); err != nil {
		return err
	}
	return nil
}

func (c *Client) ProcessStream(path string, callb func(interface{}) error, data interface{}) error {
	return c.processStream(c.BaseURL, path, c.ah, callb, data)
}

// Based on code in http://src.couchbase.org/source/xref/trunk/goproj/src/github.com/couchbase/indexing/secondary/dcp/pools.go#309
func (c *Client) processStream(baseURL *url.URL, path string, authHandler AuthHandler, callb func(interface{}) error, data interface{}) error {
	var requestUrl string

	if q := strings.Index(path, "?"); q > 0 {
		requestUrl = baseURL.Scheme + "://" + baseURL.Host + path[:q] + "?" + path[q+1:]
	} else {
		requestUrl = baseURL.Scheme + "://" + baseURL.Host + path
	}

	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return err
	}

	err = maybeAddAuth(req, authHandler)
	if err != nil {
		return err
	}

	res, err := doHTTPRequest(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != 200 {
		bod, _ := ioutil.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("HTTP error %v getting %q: %s",
			res.Status, requestUrl, bod)
	}

	reader := bufio.NewReader(res.Body)
	for {
		bs, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		if len(bs) == 1 && bs[0] == '\n' {
			continue
		}

		err = json.Unmarshal(bs, data)
		if err != nil {
			return err
		}
		err = callb(data)
		if err != nil {
			return err
		}
	}
	return nil

}

func (c *Client) parseURLResponse(path string, out interface{}) error {
	return queryRestAPI(c.BaseURL, path, c.ah, out)
}

func (c *Client) parsePostURLResponse(path string, params map[string]interface{}, out interface{}) error {
	return doPostAPI(c.BaseURL, path, params, c.ah, out)
}

func (c *Client) parsePutURLResponse(path string, params map[string]interface{}, out interface{}) error {
	return doPutAPI(c.BaseURL, path, params, c.ah, out)
}

func (b *Bucket) parseURLResponse(path string, out interface{}) error {
	nodes := b.Nodes()
	if len(nodes) == 0 {
		return errors.New("no couch rest URLs")
	}

	// Pick a random node to start querying.
	startNode := rand.Intn(len(nodes))
	maxRetries := len(nodes)
	for i := 0; i < maxRetries; i++ {
		node := nodes[(startNode+i)%len(nodes)] // Wrap around the nodes list.
		// Skip non-healthy nodes.
		if node.Status != "healthy" || node.CouchAPIBase == "" {
			continue
		}
		url := &url.URL{
			Host:   node.Hostname,
			Scheme: "http",
		}

		// Lock here to avoid having pool closed under us.
		b.RLock()
		err := queryRestAPI(url, path, b.pool.client.ah, out)
		b.RUnlock()
		if err == nil {
			return err
		}
	}
	return errors.New("All nodes failed to respond or no healthy nodes for bucket found")
}

func (b *Bucket) parseAPIResponse(path string, out interface{}) error {
	nodes := b.Nodes()
	if len(nodes) == 0 {
		return errors.New("no couch rest URLs")
	}

	var err error
	var u *url.URL

	// Pick a random node to start querying.
	startNode := rand.Intn(len(nodes))
	maxRetries := len(nodes)
	for i := 0; i < maxRetries; i++ {
		node := nodes[(startNode+i)%len(nodes)] // Wrap around the nodes list.
		// Skip non-healthy nodes.
		if node.Status != "healthy" || node.CouchAPIBase == "" {
			continue
		}

		u, err = ParseURL(node.CouchAPIBase)
		// Lock here so pool does not get closed under us.
		b.RLock()
		if err != nil {
			b.RUnlock()
			return fmt.Errorf("config error: Bucket %q node #%d CouchAPIBase=%q: %v",
				b.Name, i, node.CouchAPIBase, err)
		} else if b.pool != nil {
			u.User = b.pool.client.BaseURL.User
		}
		u.Path = path

		// generate the path so that the strings are properly escaped
		// MB-13770
		requestPath := strings.Split(u.String(), u.Host)[1]

		err = queryRestAPI(u, requestPath, b.pool.client.ah, out)
		b.RUnlock()
		if err == nil {
			return err
		}
	}

	var errStr string
	if err != nil {
		errStr = "Error " + err.Error()
	}

	return errors.New("All nodes failed to respond or returned error or no healthy nodes for bucket found." + errStr)
}

type basicAuth struct {
	u, p string
}

func (b basicAuth) GetCredentials() (string, string, string) {
	return b.u, b.p, b.u
}

func basicAuthFromURL(us string) (ah AuthHandler) {
	u, err := ParseURL(us)
	if err != nil {
		return
	}
	if user := u.User; user != nil {
		pw, _ := user.Password()
		ah = basicAuth{user.Username(), pw}
	}
	return
}

// ConnectWithAuth connects to a couchbase cluster with the given
// authentication handler.
func ConnectWithAuth(baseU string, ah AuthHandler) (c Client, err error) {
	c.BaseURL, err = ParseURL(baseU)
	if err != nil {
		return
	}
	c.ah = ah

	return c, c.parseURLResponse("/pools", &c.Info)
}

// Call this method with a TLS certificate file name to make communication
// with the KV engine encrypted.
//
// This method should be called immediately after a Connect*() method.
func (c *Client) InitTLS(certFile string) error {
	serverCert, err := ioutil.ReadFile(certFile)
	if err != nil {
		return err
	}
	CA_Pool := x509.NewCertPool()
	CA_Pool.AppendCertsFromPEM(serverCert)
	c.tlsConfig = &tls.Config{RootCAs: CA_Pool}
	return nil
}

func (c *Client) ClearTLS() {
	c.tlsConfig = nil
}

// ConnectWithAuthCreds connects to a couchbase cluster with the give
// authorization creds returned by cb_auth
func ConnectWithAuthCreds(baseU, username, password string) (c Client, err error) {
	c.BaseURL, err = ParseURL(baseU)
	if err != nil {
		return
	}

	c.ah = newBucketAuth(username, password, "")
	return c, c.parseURLResponse("/pools", &c.Info)
}

// Connect to a couchbase cluster.  An authentication handler will be
// created from the userinfo in the URL if provided.
func Connect(baseU string) (Client, error) {
	return ConnectWithAuth(baseU, basicAuthFromURL(baseU))
}

type BucketInfo struct {
	Name     string // name of bucket
	Password string // SASL password of bucket
}

//Get SASL buckets
func GetBucketList(baseU string) (bInfo []BucketInfo, err error) {

	c := &Client{}
	c.BaseURL, err = ParseURL(baseU)
	if err != nil {
		return
	}
	c.ah = basicAuthFromURL(baseU)

	var buckets []Bucket
	err = c.parseURLResponse("/pools/default/buckets", &buckets)
	if err != nil {
		return
	}
	bInfo = make([]BucketInfo, 0)
	for _, bucket := range buckets {
		bucketInfo := BucketInfo{Name: bucket.Name, Password: bucket.Password}
		bInfo = append(bInfo, bucketInfo)
	}
	return bInfo, err
}

//Set viewUpdateDaemonOptions
func SetViewUpdateParams(baseU string, params map[string]interface{}) (viewOpts map[string]interface{}, err error) {

	c := &Client{}
	c.BaseURL, err = ParseURL(baseU)
	if err != nil {
		return
	}
	c.ah = basicAuthFromURL(baseU)

	if len(params) < 1 {
		return nil, fmt.Errorf("No params to set")
	}

	err = c.parsePostURLResponse("/settings/viewUpdateDaemon", params, &viewOpts)
	if err != nil {
		return
	}
	return viewOpts, err
}

// This API lets the caller know, if the list of nodes a bucket is
// connected to has gone through an edit (a rebalance operation)
// since the last update to the bucket, in which case a Refresh is
// advised.
func (b *Bucket) NodeListChanged() bool {
	b.RLock()
	pool := b.pool
	uri := b.URI
	b.RUnlock()

	tmpb := &Bucket{}
	err := pool.client.parseURLResponse(uri, tmpb)
	if err != nil {
		return true
	}

	bNodes := *(*[]Node)(b.nodeList)
	if len(bNodes) != len(tmpb.NodesJSON) {
		return true
	}

	bucketHostnames := map[string]bool{}
	for _, node := range bNodes {
		bucketHostnames[node.Hostname] = true
	}

	for _, node := range tmpb.NodesJSON {
		if _, found := bucketHostnames[node.Hostname]; !found {
			return true
		}
	}

	return false
}

// Sample data for scopes and collections as returned from the
// /pooles/default/$BUCKET_NAME/collections API.
// {"myScope2":{"myCollectionC":{}},"myScope1":{"myCollectionB":{},"myCollectionA":{}},"_default":{"_default":{}}}

// Structures for parsing collections manifest.
// The map key is the name of the scope.
// Example data:
// {"uid":"b","scopes":[
//    {"name":"_default","uid":"0","collections":[
//       {"name":"_default","uid":"0"}]},
//    {"name":"myScope1","uid":"8","collections":[
//       {"name":"myCollectionB","uid":"c"},
//       {"name":"myCollectionA","uid":"b"}]},
//    {"name":"myScope2","uid":"9","collections":[
//       {"name":"myCollectionC","uid":"d"}]}]}
type InputManifest struct {
	Uid    string
	Scopes []InputScope
}
type InputScope struct {
	Name        string
	Uid         string
	Collections []InputCollection
}
type InputCollection struct {
	Name string
	Uid  string
}

// Structures for storing collections information.
type Manifest struct {
	Uid    uint64
	Scopes map[string]*Scope // map by name
}
type Scope struct {
	Name        string
	Uid         uint64
	Collections map[string]*Collection // map by name
}
type Collection struct {
	Name string
	Uid  uint64
}

var _EMPTY_MANIFEST *Manifest = &Manifest{Uid: 0, Scopes: map[string]*Scope{}}

func parseCollectionsManifest(res *gomemcached.MCResponse) (*Manifest, error) {
	if !EnableCollections {
		return _EMPTY_MANIFEST, nil
	}

	var im InputManifest
	err := json.Unmarshal(res.Body, &im)
	if err != nil {
		return nil, err
	}

	uid, err := strconv.ParseUint(im.Uid, 16, 64)
	if err != nil {
		return nil, err
	}
	mani := &Manifest{Uid: uid, Scopes: make(map[string]*Scope, len(im.Scopes))}
	for _, iscope := range im.Scopes {
		scope_uid, err := strconv.ParseUint(iscope.Uid, 16, 64)
		if err != nil {
			return nil, err
		}
		scope := &Scope{Uid: scope_uid, Name: iscope.Name, Collections: make(map[string]*Collection, len(iscope.Collections))}
		mani.Scopes[iscope.Name] = scope
		for _, icoll := range iscope.Collections {
			coll_uid, err := strconv.ParseUint(icoll.Uid, 16, 64)
			if err != nil {
				return nil, err
			}
			coll := &Collection{Uid: coll_uid, Name: icoll.Name}
			scope.Collections[icoll.Name] = coll
		}
	}

	return mani, nil
}

// This function assumes the bucket is locked.
func (b *Bucket) GetCollectionsManifest() (*Manifest, error) {
	// Collections not used?
	if !EnableCollections {
		return nil, fmt.Errorf("Collections not enabled.")
	}

	b.RLock()
	pools := b.getConnPools(true /* already locked */)
	pool := pools[0] // Any pool will do, so use the first one.
	b.RUnlock()
	client, err := pool.Get()
	if err != nil {
		return nil, fmt.Errorf("Unable to get connection to retrieve collections manifest: %v. No collections access to bucket %s.", err, b.Name)
	}

	// We need to select the bucket before GetCollectionsManifest()
	// will work. This is sometimes done at startup (see defaultMkConn())
	// but not always, depending on the auth type.
	// Doing this is safe because we collect the the connections
	// by bucket, so the bucket being selected will never change.
	_, err = client.SelectBucket(b.Name)
	if err != nil {
		pool.Return(client)
		return nil, fmt.Errorf("Unable to select bucket %s: %v. No collections access to bucket %s.", err, b.Name, b.Name)
	}

	res, err := client.GetCollectionsManifest()
	if err != nil {
		pool.Return(client)
		return nil, fmt.Errorf("Unable to retrieve collections manifest: %v. No collections access to bucket %s.", err, b.Name)
	}
	mani, err := parseCollectionsManifest(res)
	if err != nil {
		pool.Return(client)
		return nil, fmt.Errorf("Unable to parse collections manifest: %v. No collections access to bucket %s.", err, b.Name)
	}

	pool.Return(client)
	return mani, nil
}

func (b *Bucket) RefreshFully() error {
	return b.refresh(false)
}

func (b *Bucket) Refresh() error {
	return b.refresh(true)
}

func (b *Bucket) refresh(preserveConnections bool) error {
	b.RLock()
	pool := b.pool
	uri := b.URI
	client := pool.client
	b.RUnlock()
	tlsConfig := client.tlsConfig

	var poolServices PoolServices
	var err error
	if tlsConfig != nil {
		poolServices, err = client.GetPoolServices("default")
		if err != nil {
			return err
		}
	}

	tmpb := &Bucket{}
	err = pool.client.parseURLResponse(uri, tmpb)
	if err != nil {
		return err
	}

	pools := b.getConnPools(false /* bucket not already locked */)

	// We need this lock to ensure that bucket refreshes happening because
	// of NMVb errors received during bulkGet do not end up over-writing
	// pool.inUse.
	b.Lock()

	for _, pool := range pools {
		if pool != nil {
			pool.inUse = false
		}
	}

	newcps := make([]*connectionPool, len(tmpb.VBSMJson.ServerList))
	for i := range newcps {

		if preserveConnections {
			pool := b.getConnPoolByHost(tmpb.VBSMJson.ServerList[i], true /* bucket already locked */)
			if pool != nil && pool.inUse == false {
				// if the hostname and index is unchanged then reuse this pool
				newcps[i] = pool
				pool.inUse = true
				continue
			}
		}

		hostport := tmpb.VBSMJson.ServerList[i]
		if tlsConfig != nil {
			hostport, err = MapKVtoSSL(hostport, &poolServices)
			if err != nil {
				b.Unlock()
				return err
			}
		}

		if b.ah != nil {
			newcps[i] = newConnectionPool(hostport,
				b.ah, AsynchronousCloser, PoolSize, PoolOverflow, tlsConfig, b.Name)

		} else {
			newcps[i] = newConnectionPool(hostport,
				b.authHandler(true /* bucket already locked */),
				AsynchronousCloser, PoolSize, PoolOverflow, tlsConfig, b.Name)
		}
	}
	b.replaceConnPools2(newcps, true /* bucket already locked */)
	tmpb.ah = b.ah
	b.vBucketServerMap = unsafe.Pointer(&tmpb.VBSMJson)
	b.nodeList = unsafe.Pointer(&tmpb.NodesJSON)

	b.Unlock()
	return nil
}

func (p *Pool) refresh() (err error) {
	p.BucketMap = make(map[string]*Bucket)

	buckets := []Bucket{}
	err = p.client.parseURLResponse(p.BucketURL["uri"], &buckets)
	if err != nil {
		return err
	}
	for i, _ := range buckets {
		b := new(Bucket)
		*b = buckets[i]
		b.pool = p
		b.nodeList = unsafe.Pointer(&b.NodesJSON)

		// MB-33185 this is merely defensive, just in case
		// refresh() gets called on a perfectly node pool
		ob, ok := p.BucketMap[b.Name]
		if ok && ob.connPools != nil {
			ob.Close()
		}
		b.replaceConnPools(make([]*connectionPool, len(b.VBSMJson.ServerList)))
		p.BucketMap[b.Name] = b
		runtime.SetFinalizer(b, bucketFinalizer)
	}
	return nil
}

// GetPool gets a pool from within the couchbase cluster (usually
// "default").
func (c *Client) GetPool(name string) (p Pool, err error) {
	var poolURI string

	for _, p := range c.Info.Pools {
		if p.Name == name {
			poolURI = p.URI
			break
		}
	}
	if poolURI == "" {
		return p, errors.New("No pool named " + name)
	}

	err = c.parseURLResponse(poolURI, &p)

	p.client = c

	err = p.refresh()
	return
}

// GetPoolServices returns all the bucket-independent services in a pool.
// (See "Exposing services outside of bucket context" in http://goo.gl/uuXRkV)
func (c *Client) GetPoolServices(name string) (ps PoolServices, err error) {
	var poolName string
	for _, p := range c.Info.Pools {
		if p.Name == name {
			poolName = p.Name
		}
	}
	if poolName == "" {
		return ps, errors.New("No pool named " + name)
	}

	poolURI := "/pools/" + poolName + "/nodeServices"
	err = c.parseURLResponse(poolURI, &ps)

	return
}

func (b *Bucket) GetPoolServices(name string) (*PoolServices, error) {
	b.RLock()
	pool := b.pool
	b.RUnlock()

	ps, err := pool.client.GetPoolServices(name)
	if err != nil {
		return nil, err
	}

	return &ps, nil
}

// Close marks this bucket as no longer needed, closing connections it
// may have open.
func (b *Bucket) Close() {
	b.Lock()
	defer b.Unlock()
	if b.connPools != nil {
		for _, c := range b.getConnPools(true /* already locked */) {
			if c != nil {
				c.Close()
			}
		}
		b.connPools = nil
	}
}

func bucketFinalizer(b *Bucket) {
	if b.connPools != nil {
		if !b.closed {
			logging.Warnf("Finalizing a bucket with active connections.")
		}

		// MB-33185 do not leak connection pools
		b.Close()
	}
}

// GetBucket gets a bucket from within this pool.
func (p *Pool) GetBucket(name string) (*Bucket, error) {
	rv, ok := p.BucketMap[name]
	if !ok {
		return nil, &BucketNotFoundError{bucket: name}
	}
	err := rv.Refresh()
	if err != nil {
		return nil, err
	}
	return rv, nil
}

// GetBucket gets a bucket from within this pool.
func (p *Pool) GetBucketWithAuth(bucket, username, password string) (*Bucket, error) {
	rv, ok := p.BucketMap[bucket]
	if !ok {
		return nil, &BucketNotFoundError{bucket: bucket}
	}
	rv.ah = newBucketAuth(username, password, bucket)
	err := rv.Refresh()
	if err != nil {
		return nil, err
	}
	return rv, nil
}

// GetPool gets the pool to which this bucket belongs.
func (b *Bucket) GetPool() *Pool {
	b.RLock()
	defer b.RUnlock()
	ret := b.pool
	return ret
}

// GetClient gets the client from which we got this pool.
func (p *Pool) GetClient() *Client {
	return p.client
}

// Release bucket connections when the pool is no longer in use
func (p *Pool) Close() {
	// fine to loop through the buckets unlocked
	// locking happens at the bucket level
	for b, _ := range p.BucketMap {

		// MB-33208 defer closing connection pools until the bucket is no longer used
		bucket := p.BucketMap[b]
		bucket.Lock()
		bucket.closed = true
		bucket.Unlock()
	}
}

// GetBucket is a convenience function for getting a named bucket from
// a URL
func GetBucket(endpoint, poolname, bucketname string) (*Bucket, error) {
	var err error
	client, err := Connect(endpoint)
	if err != nil {
		return nil, err
	}

	pool, err := client.GetPool(poolname)
	if err != nil {
		return nil, err
	}

	return pool.GetBucket(bucketname)
}

// ConnectWithAuthAndGetBucket is a convenience function for
// getting a named bucket from a given URL and an auth callback
func ConnectWithAuthAndGetBucket(endpoint, poolname, bucketname string,
	ah AuthHandler) (*Bucket, error) {
	client, err := ConnectWithAuth(endpoint, ah)
	if err != nil {
		return nil, err
	}

	pool, err := client.GetPool(poolname)
	if err != nil {
		return nil, err
	}

	return pool.GetBucket(bucketname)
}
