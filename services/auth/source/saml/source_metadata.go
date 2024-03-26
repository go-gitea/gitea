// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import (
	"encoding/xml"
	"fmt"
	"net/http"
)

// Metadata redirects request/response pair to authenticate against the provider
func (source *Source) Metadata(request *http.Request, response http.ResponseWriter) error {
	samlRWMutex.RLock()
	defer samlRWMutex.RUnlock()
	if _, ok := providers[source.authSource.Name]; !ok {
		return fmt.Errorf("provider does not exist")
	}

	metadata, err := providers[source.authSource.Name].samlSP.Metadata()
	if err != nil {
		return err
	}
	buf, err := xml.Marshal(metadata)
	if err != nil {
		return err
	}

	response.Header().Set("Content-Type", "application/samlmetadata+xml; charset=utf-8")
	_, _ = response.Write(buf)
	return nil
}
