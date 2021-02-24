package couchbase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/couchbase/goutils/logging"
	"io/ioutil"
	"net/http"
)

// ViewDefinition represents a single view within a design document.
type ViewDefinition struct {
	Map    string `json:"map"`
	Reduce string `json:"reduce,omitempty"`
}

// DDoc is the document body of a design document specifying a view.
type DDoc struct {
	Language string                    `json:"language,omitempty"`
	Views    map[string]ViewDefinition `json:"views"`
}

// DDocsResult represents the result from listing the design
// documents.
type DDocsResult struct {
	Rows []struct {
		DDoc struct {
			Meta map[string]interface{}
			JSON DDoc
		} `json:"doc"`
	} `json:"rows"`
}

// GetDDocs lists all design documents
func (b *Bucket) GetDDocs() (DDocsResult, error) {
	var ddocsResult DDocsResult
	b.RLock()
	pool := b.pool
	uri := b.DDocs.URI
	b.RUnlock()

	// MB-23555 ephemeral buckets have no ddocs
	if uri == "" {
		return DDocsResult{}, nil
	}

	err := pool.client.parseURLResponse(uri, &ddocsResult)
	if err != nil {
		return DDocsResult{}, err
	}
	return ddocsResult, nil
}

func (b *Bucket) GetDDocWithRetry(docname string, into interface{}) error {
	ddocURI := fmt.Sprintf("/%s/_design/%s", b.GetName(), docname)
	err := b.parseAPIResponse(ddocURI, &into)
	if err != nil {
		return err
	}
	return nil
}

func (b *Bucket) GetDDocsWithRetry() (DDocsResult, error) {
	var ddocsResult DDocsResult
	b.RLock()
	uri := b.DDocs.URI
	b.RUnlock()

	// MB-23555 ephemeral buckets have no ddocs
	if uri == "" {
		return DDocsResult{}, nil
	}

	err := b.parseURLResponse(uri, &ddocsResult)
	if err != nil {
		return DDocsResult{}, err
	}
	return ddocsResult, nil
}

func (b *Bucket) ddocURL(docname string) (string, error) {
	u, err := b.randomBaseURL()
	if err != nil {
		return "", err
	}
	u.Path = fmt.Sprintf("/%s/_design/%s", b.GetName(), docname)
	return u.String(), nil
}

func (b *Bucket) ddocURLNext(nodeId int, docname string) (string, int, error) {
	u, selected, err := b.randomNextURL(nodeId)
	if err != nil {
		return "", -1, err
	}
	u.Path = fmt.Sprintf("/%s/_design/%s", b.GetName(), docname)
	return u.String(), selected, nil
}

const ABS_MAX_RETRIES = 10
const ABS_MIN_RETRIES = 3

func (b *Bucket) getMaxRetries() (int, error) {

	maxRetries := len(b.Nodes())

	if maxRetries == 0 {
		return 0, fmt.Errorf("No available Couch rest URLs")
	}

	if maxRetries > ABS_MAX_RETRIES {
		maxRetries = ABS_MAX_RETRIES
	} else if maxRetries < ABS_MIN_RETRIES {
		maxRetries = ABS_MIN_RETRIES
	}

	return maxRetries, nil
}

// PutDDoc installs a design document.
func (b *Bucket) PutDDoc(docname string, value interface{}) error {

	var Err error

	maxRetries, err := b.getMaxRetries()
	if err != nil {
		return err
	}

	lastNode := START_NODE_ID

	for retryCount := 0; retryCount < maxRetries; retryCount++ {

		Err = nil

		ddocU, selectedNode, err := b.ddocURLNext(lastNode, docname)
		if err != nil {
			return err
		}

		lastNode = selectedNode

		logging.Infof(" Trying with selected node %d", selectedNode)
		j, err := json.Marshal(value)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("PUT", ddocU, bytes.NewReader(j))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		err = maybeAddAuth(req, b.authHandler(false /* bucket not yet locked */))
		if err != nil {
			return err
		}

		res, err := doHTTPRequest(req)
		if err != nil {
			return err
		}

		if res.StatusCode != 201 {
			body, _ := ioutil.ReadAll(res.Body)
			Err = fmt.Errorf("error installing view: %v / %s",
				res.Status, body)
			logging.Errorf(" Error in PutDDOC %v. Retrying...", Err)
			res.Body.Close()
			b.Refresh()
			continue
		}

		res.Body.Close()
		break
	}

	return Err
}

// GetDDoc retrieves a specific a design doc.
func (b *Bucket) GetDDoc(docname string, into interface{}) error {
	var Err error
	var res *http.Response

	maxRetries, err := b.getMaxRetries()
	if err != nil {
		return err
	}

	lastNode := START_NODE_ID
	for retryCount := 0; retryCount < maxRetries; retryCount++ {

		Err = nil
		ddocU, selectedNode, err := b.ddocURLNext(lastNode, docname)
		if err != nil {
			return err
		}

		lastNode = selectedNode
		logging.Infof(" Trying with selected node %d", selectedNode)

		req, err := http.NewRequest("GET", ddocU, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		err = maybeAddAuth(req, b.authHandler(false /* bucket not yet locked */))
		if err != nil {
			return err
		}

		res, err = doHTTPRequest(req)
		if err != nil {
			return err
		}
		if res.StatusCode != 200 {
			body, _ := ioutil.ReadAll(res.Body)
			Err = fmt.Errorf("error reading view: %v / %s",
				res.Status, body)
			logging.Errorf(" Error in GetDDOC %v Retrying...", Err)
			b.Refresh()
			res.Body.Close()
			continue
		}
		defer res.Body.Close()
		break
	}

	if Err != nil {
		return Err
	}

	d := json.NewDecoder(res.Body)
	return d.Decode(into)
}

// DeleteDDoc removes a design document.
func (b *Bucket) DeleteDDoc(docname string) error {

	var Err error

	maxRetries, err := b.getMaxRetries()
	if err != nil {
		return err
	}

	lastNode := START_NODE_ID

	for retryCount := 0; retryCount < maxRetries; retryCount++ {

		Err = nil
		ddocU, selectedNode, err := b.ddocURLNext(lastNode, docname)
		if err != nil {
			return err
		}

		lastNode = selectedNode
		logging.Infof(" Trying with selected node %d", selectedNode)

		req, err := http.NewRequest("DELETE", ddocU, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		err = maybeAddAuth(req, b.authHandler(false /* bucket not already locked */))
		if err != nil {
			return err
		}

		res, err := doHTTPRequest(req)
		if err != nil {
			return err
		}
		if res.StatusCode != 200 {
			body, _ := ioutil.ReadAll(res.Body)
			Err = fmt.Errorf("error deleting view : %v / %s", res.Status, body)
			logging.Errorf(" Error in DeleteDDOC %v. Retrying ... ", Err)
			b.Refresh()
			res.Body.Close()
			continue
		}

		res.Body.Close()
		break
	}
	return Err
}
