package openid

// 7.3.1.  Discovered Information
// Upon successful completion of discovery, the Relying Party will
// have one or more sets of the following information (see the
// Terminology section for definitions). If more than one set of the
// following information has been discovered, the precedence rules
// defined in [XRI_Resolution_2.0] are to be applied.
//   - OP Endpoint URL
//   - Protocol Version
// If the end user did not enter an OP Identifier, the following
// information will also be present:
//   - Claimed Identifier
//   - OP-Local Identifier
// If the end user entered an OP Identifier, there is no Claimed
// Identifier. For the purposes of making OpenID Authentication
// requests, the value
// "http://specs.openid.net/auth/2.0/identifier_select" MUST be
// used as both the Claimed Identifier and the OP-Local Identifier
// when an OP Identifier is entered.
func Discover(id string) (opEndpoint, opLocalID, claimedID string, err error) {
	return defaultInstance.Discover(id)
}

func (oid *OpenID) Discover(id string) (opEndpoint, opLocalID, claimedID string, err error) {
	// From OpenID specs, 7.2: Normalization
	if id, err = Normalize(id); err != nil {
		return
	}

	// From OpenID specs, 7.3: Discovery.

	// If the identifier is an XRI, [XRI_Resolution_2.0] will yield an
	// XRDS document that contains the necessary information. It
	// should also be noted that Relying Parties can take advantage of
	// XRI Proxy Resolvers, such as the one provided by XDI.org at
	// http://www.xri.net. This will remove the need for the RPs to
	// perform XRI Resolution locally.

	// XRI not supported.

	// If it is a URL, the Yadis protocol [Yadis] SHALL be first
	// attempted. If it succeeds, the result is again an XRDS
	// document.
	if opEndpoint, opLocalID, err = yadisDiscovery(id, oid.urlGetter); err != nil {
		// If the Yadis protocol fails and no valid XRDS document is
		// retrieved, or no Service Elements are found in the XRDS
		// document, the URL is retrieved and HTML-Based discovery SHALL be
		// attempted.
		opEndpoint, opLocalID, claimedID, err = htmlDiscovery(id, oid.urlGetter)
	}

	if err != nil {
		return "", "", "", err
	}
	return
}
