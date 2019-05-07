package couchbase

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

// ViewRow represents a single result from a view.
//
// Doc is present only if include_docs was set on the request.
type ViewRow struct {
	ID    string
	Key   interface{}
	Value interface{}
	Doc   *interface{}
}

// A ViewError is a node-specific error indicating a partial failure
// within a view result.
type ViewError struct {
	From   string
	Reason string
}

func (ve ViewError) Error() string {
	return "Node: " + ve.From + ", reason: " + ve.Reason
}

// ViewResult holds the entire result set from a view request,
// including the rows and the errors.
type ViewResult struct {
	TotalRows int `json:"total_rows"`
	Rows      []ViewRow
	Errors    []ViewError
}

func (b *Bucket) randomBaseURL() (*url.URL, error) {
	nodes := b.HealthyNodes()
	if len(nodes) == 0 {
		return nil, errors.New("no available couch rest URLs")
	}
	nodeNo := rand.Intn(len(nodes))
	node := nodes[nodeNo]

	b.RLock()
	name := b.Name
	pool := b.pool
	b.RUnlock()

	u, err := ParseURL(node.CouchAPIBase)
	if err != nil {
		return nil, fmt.Errorf("config error: Bucket %q node #%d CouchAPIBase=%q: %v",
			name, nodeNo, node.CouchAPIBase, err)
	} else if pool != nil {
		u.User = pool.client.BaseURL.User
	}
	return u, err
}

const START_NODE_ID = -1

func (b *Bucket) randomNextURL(lastNode int) (*url.URL, int, error) {
	nodes := b.HealthyNodes()
	if len(nodes) == 0 {
		return nil, -1, errors.New("no available couch rest URLs")
	}

	var nodeNo int
	if lastNode == START_NODE_ID || lastNode >= len(nodes) {
		// randomly select a node if the value of lastNode is invalid
		nodeNo = rand.Intn(len(nodes))
	} else {
		// wrap around the node list
		nodeNo = (lastNode + 1) % len(nodes)
	}

	b.RLock()
	name := b.Name
	pool := b.pool
	b.RUnlock()

	node := nodes[nodeNo]
	u, err := ParseURL(node.CouchAPIBase)
	if err != nil {
		return nil, -1, fmt.Errorf("config error: Bucket %q node #%d CouchAPIBase=%q: %v",
			name, nodeNo, node.CouchAPIBase, err)
	} else if pool != nil {
		u.User = pool.client.BaseURL.User
	}
	return u, nodeNo, err
}

// DocID is the document ID type for the startkey_docid parameter in
// views.
type DocID string

func qParam(k, v string) string {
	format := `"%s"`
	switch k {
	case "startkey_docid", "endkey_docid", "stale":
		format = "%s"
	}
	return fmt.Sprintf(format, v)
}

// ViewURL constructs a URL for a view with the given ddoc, view name,
// and parameters.
func (b *Bucket) ViewURL(ddoc, name string,
	params map[string]interface{}) (string, error) {
	u, err := b.randomBaseURL()
	if err != nil {
		return "", err
	}

	values := url.Values{}
	for k, v := range params {
		switch t := v.(type) {
		case DocID:
			values[k] = []string{string(t)}
		case string:
			values[k] = []string{qParam(k, t)}
		case int:
			values[k] = []string{fmt.Sprintf(`%d`, t)}
		case bool:
			values[k] = []string{fmt.Sprintf(`%v`, t)}
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return "", fmt.Errorf("unsupported value-type %T in Query, "+
					"json encoder said %v", t, err)
			}
			values[k] = []string{fmt.Sprintf(`%v`, string(b))}
		}
	}

	if ddoc == "" && name == "_all_docs" {
		u.Path = fmt.Sprintf("/%s/_all_docs", b.GetName())
	} else {
		u.Path = fmt.Sprintf("/%s/_design/%s/_view/%s", b.GetName(), ddoc, name)
	}
	u.RawQuery = values.Encode()

	return u.String(), nil
}

// ViewCallback is called for each view invocation.
var ViewCallback func(ddoc, name string, start time.Time, err error)

// ViewCustom performs a view request that can map row values to a
// custom type.
//
// See the source to View for an example usage.
func (b *Bucket) ViewCustom(ddoc, name string, params map[string]interface{},
	vres interface{}) (err error) {
	if SlowServerCallWarningThreshold > 0 {
		defer slowLog(time.Now(), "call to ViewCustom(%q, %q)", ddoc, name)
	}

	if ViewCallback != nil {
		defer func(t time.Time) { ViewCallback(ddoc, name, t, err) }(time.Now())
	}

	u, err := b.ViewURL(ddoc, name, params)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}

	ah := b.authHandler(false /* bucket not yet locked */)
	maybeAddAuth(req, ah)

	res, err := doHTTPRequest(req)
	if err != nil {
		return fmt.Errorf("error starting view req at %v: %v", u, err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		bod := make([]byte, 512)
		l, _ := res.Body.Read(bod)
		return fmt.Errorf("error executing view req at %v: %v - %s",
			u, res.Status, bod[:l])
	}

	body, err := ioutil.ReadAll(res.Body)
	if err := json.Unmarshal(body, vres); err != nil {
		return nil
	}

	return nil
}

// View executes a view.
//
// The ddoc parameter is just the bare name of your design doc without
// the "_design/" prefix.
//
// Parameters are string keys with values that correspond to couchbase
// view parameters.  Primitive should work fairly naturally (booleans,
// ints, strings, etc...) and other values will attempt to be JSON
// marshaled (useful for array indexing on on view keys, for example).
//
// Example:
//
//   res, err := couchbase.View("myddoc", "myview", map[string]interface{}{
//       "group_level": 2,
//       "startkey_docid":    []interface{}{"thing"},
//       "endkey_docid":      []interface{}{"thing", map[string]string{}},
//       "stale": false,
//       })
func (b *Bucket) View(ddoc, name string, params map[string]interface{}) (ViewResult, error) {
	vres := ViewResult{}

	if err := b.ViewCustom(ddoc, name, params, &vres); err != nil {
		//error in accessing views. Retry once after a bucket refresh
		b.Refresh()
		return vres, b.ViewCustom(ddoc, name, params, &vres)
	} else {
		return vres, nil
	}
}
