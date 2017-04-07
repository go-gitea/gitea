package openid

import (
	"encoding/xml"
	"errors"
	"strings"
)

// TODO: As per 11.2 in openid 2 specs, a service may have multiple
//       URIs. We don't care for discovery really, but we do care for
//       verification though.
type XrdsIdentifier struct {
	Type     []string `xml:"Type"`
	URI      string   `xml:"URI"`
	LocalID  string   `xml:"LocalID"`
	Priority int      `xml:"priority,attr"`
}

type Xrd struct {
	Service []*XrdsIdentifier `xml:"Service"`
}

type XrdsDocument struct {
	XMLName xml.Name `xml:"XRDS"`
	Xrd     *Xrd     `xml:"XRD"`
}

func parseXrds(input []byte) (opEndpoint, opLocalID string, err error) {
	xrdsDoc := &XrdsDocument{}
	err = xml.Unmarshal(input, xrdsDoc)
	if err != nil {
		return
	}

	if xrdsDoc.Xrd == nil {
		return "", "", errors.New("XRDS document missing XRD tag")
	}

	// 7.3.2.2.  Extracting Authentication Data
	// Once the Relying Party has obtained an XRDS document, it
	// MUST first search the document (following the rules
	// described in [XRI_Resolution_2.0]) for an OP Identifier
	// Element. If none is found, the RP will search for a Claimed
	// Identifier Element.
	for _, service := range xrdsDoc.Xrd.Service {
		// 7.3.2.1.1.  OP Identifier Element
		// An OP Identifier Element is an <xrd:Service> element with the
		// following information:
		// An <xrd:Type> tag whose text content is
		//     "http://specs.openid.net/auth/2.0/server".
		// An <xrd:URI> tag whose text content is the OP Endpoint URL
		if service.hasType("http://specs.openid.net/auth/2.0/server") {
			opEndpoint = strings.TrimSpace(service.URI)
			return
		}
	}
	for _, service := range xrdsDoc.Xrd.Service {
		// 7.3.2.1.2.  Claimed Identifier Element
		// A Claimed Identifier Element is an <xrd:Service> element
		// with the following information:
		// An <xrd:Type> tag whose text content is
		//     "http://specs.openid.net/auth/2.0/signon".
		// An <xrd:URI> tag whose text content is the OP Endpoint
		//     URL.
		// An <xrd:LocalID> tag (optional) whose text content is the
		//     OP-Local Identifier.
		if service.hasType("http://specs.openid.net/auth/2.0/signon") {
			opEndpoint = strings.TrimSpace(service.URI)
			opLocalID = strings.TrimSpace(service.LocalID)
			return
		}
	}
	return "", "", errors.New("Could not find a compatible service")
}

func (xrdsi *XrdsIdentifier) hasType(tpe string) bool {
	for _, t := range xrdsi.Type {
		if t == tpe {
			return true
		}
	}
	return false
}
