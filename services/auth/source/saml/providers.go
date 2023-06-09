package saml

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
)

// Providers is list of known/available providers.
type Providers map[string]Source

var providers = Providers{}

// used to create different types of goth providers
func createProvider(ctx context.Context, source *Source) (*Source, error) {
	source.CallbackURL = setting.AppURL + "user/saml/" + url.PathEscape(source.authSource.Name) + "/acs"

	idpMetadata, err := readIdentityProviderMetadata(ctx, source)
	if err != nil {
		return source, err
	}
	{
		if source.IdentityProviderMetadataURL != "" {
			log.Trace(fmt.Sprintf("Identity Provider metadata: %s", source.IdentityProviderMetadataURL), string(idpMetadata))
		}
	}

	metadata := &types.EntityDescriptor{}
	err = xml.Unmarshal(idpMetadata, metadata)
	if err != nil {
		return source, err
	}

	certStore := dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{},
	}

	for _, kd := range metadata.IDPSSODescriptor.KeyDescriptors {
		for idx, xcert := range kd.KeyInfo.X509Data.X509Certificates {
			if xcert.Data == "" {
				return source, fmt.Errorf("metadata certificate(%d) must not be empty", idx)
			}
			certData, err := base64.StdEncoding.DecodeString(xcert.Data)
			if err != nil {
				return source, err
			}

			idpCert, err := x509.ParseCertificate(certData)
			if err != nil {
				return source, err
			}

			certStore.Roots = append(certStore.Roots, idpCert)
		}
	}

	var keyStore dsig.X509KeyStore

	if source.ServiceProviderCertificate != "" && source.ServiceProviderPrivateKey != "" {
		keyPair, err := tls.X509KeyPair([]byte(source.ServiceProviderCertificate), []byte(source.ServiceProviderPrivateKey))
		if err != nil {
			return nil, err
		}
		keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
		if err != nil {
			return nil, err
		}
		keyStore = dsig.TLSCertKeyStore(keyPair)
	} else {
		keyStore = dsig.RandomKeyStoreForTest()
	}

	source.samlSP = &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:      metadata.IDPSSODescriptor.SingleSignOnServices[0].Location,
		IdentityProviderIssuer:      metadata.EntityID,
		AudienceURI:                 setting.AppURL + "user/saml/" + url.PathEscape(source.authSource.Name) + "/metadata",
		AssertionConsumerServiceURL: source.CallbackURL,
		SignAuthnRequests:           !source.InsecureSkipAssertionSignatureValidation,
		NameIdFormat:                source.NameIDFormat.String(),
		IDPCertificateStore:         &certStore,
		SPKeyStore:                  keyStore,
		ServiceProviderIssuer:       setting.AppURL + "user/saml/" + url.PathEscape(source.authSource.Name) + "/metadata",
	}

	return source, err
}

func readIdentityProviderMetadata(ctx context.Context, source *Source) ([]byte, error) {
	if source.IdentityProviderMetadata != "" {
		return []byte(source.IdentityProviderMetadata), nil
	}

	req := httplib.NewRequest(source.IdentityProviderMetadataURL, "GET")
	req.SetTimeout(20*time.Second, time.Minute)
	resp, err := req.Response()
	if err != nil {
		return nil, fmt.Errorf("Unable to contact gitea: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
