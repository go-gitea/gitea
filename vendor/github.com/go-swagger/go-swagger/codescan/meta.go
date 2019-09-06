// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codescan

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"net/mail"
	"regexp"
	"strings"

	"github.com/go-openapi/spec"
)

type metaSection struct {
	Comments *ast.CommentGroup
}

func metaTOSSetter(meta *spec.Info) func([]string) {
	return func(lines []string) {
		meta.TermsOfService = joinDropLast(lines)
	}
}

func metaConsumesSetter(meta *spec.Swagger) func([]string) {
	return func(consumes []string) { meta.Consumes = consumes }
}

func metaProducesSetter(meta *spec.Swagger) func([]string) {
	return func(produces []string) { meta.Produces = produces }
}

func metaSchemeSetter(meta *spec.Swagger) func([]string) {
	return func(schemes []string) { meta.Schemes = schemes }
}

func metaSecuritySetter(meta *spec.Swagger) func([]map[string][]string) {
	return func(secDefs []map[string][]string) { meta.Security = secDefs }
}

func metaSecurityDefinitionsSetter(meta *spec.Swagger) func(json.RawMessage) error {
	return func(jsonValue json.RawMessage) error {
		var jsonData spec.SecurityDefinitions
		err := json.Unmarshal(jsonValue, &jsonData)
		if err != nil {
			return err
		}
		meta.SecurityDefinitions = jsonData
		return nil
	}
}

func metaVendorExtensibleSetter(meta *spec.Swagger) func(json.RawMessage) error {
	return func(jsonValue json.RawMessage) error {
		var jsonData spec.Extensions
		err := json.Unmarshal(jsonValue, &jsonData)
		if err != nil {
			return err
		}
		for k := range jsonData {
			if !rxAllowedExtensions.MatchString(k) {
				return fmt.Errorf("invalid schema extension name, should start from `x-`: %s", k)
			}
		}
		meta.Extensions = jsonData
		return nil
	}
}

func infoVendorExtensibleSetter(meta *spec.Swagger) func(json.RawMessage) error {
	return func(jsonValue json.RawMessage) error {
		var jsonData spec.Extensions
		err := json.Unmarshal(jsonValue, &jsonData)
		if err != nil {
			return err
		}
		for k := range jsonData {
			if !rxAllowedExtensions.MatchString(k) {
				return fmt.Errorf("invalid schema extension name, should start from `x-`: %s", k)
			}
		}
		meta.Info.Extensions = jsonData
		return nil
	}
}

func newMetaParser(swspec *spec.Swagger) *sectionedParser {
	sp := new(sectionedParser)
	if swspec.Info == nil {
		swspec.Info = new(spec.Info)
	}
	info := swspec.Info
	sp.setTitle = func(lines []string) {
		tosave := joinDropLast(lines)
		if len(tosave) > 0 {
			tosave = rxStripTitleComments.ReplaceAllString(tosave, "")
		}
		info.Title = tosave
	}
	sp.setDescription = func(lines []string) { info.Description = joinDropLast(lines) }
	sp.taggers = []tagParser{
		newMultiLineTagParser("TOS", newMultilineDropEmptyParser(rxTOS, metaTOSSetter(info)), false),
		newMultiLineTagParser("Consumes", newMultilineDropEmptyParser(rxConsumes, metaConsumesSetter(swspec)), false),
		newMultiLineTagParser("Produces", newMultilineDropEmptyParser(rxProduces, metaProducesSetter(swspec)), false),
		newSingleLineTagParser("Schemes", newSetSchemes(metaSchemeSetter(swspec))),
		newMultiLineTagParser("Security", newSetSecurity(rxSecuritySchemes, metaSecuritySetter(swspec)), false),
		newMultiLineTagParser("SecurityDefinitions", newYamlParser(rxSecurity, metaSecurityDefinitionsSetter(swspec)), true),
		newSingleLineTagParser("Version", &setMetaSingle{swspec, rxVersion, setInfoVersion}),
		newSingleLineTagParser("Host", &setMetaSingle{swspec, rxHost, setSwaggerHost}),
		newSingleLineTagParser("BasePath", &setMetaSingle{swspec, rxBasePath, setSwaggerBasePath}),
		newSingleLineTagParser("Contact", &setMetaSingle{swspec, rxContact, setInfoContact}),
		newSingleLineTagParser("License", &setMetaSingle{swspec, rxLicense, setInfoLicense}),
		newMultiLineTagParser("YAMLInfoExtensionsBlock", newYamlParser(rxInfoExtensions, infoVendorExtensibleSetter(swspec)), true),
		newMultiLineTagParser("YAMLExtensionsBlock", newYamlParser(rxExtensions, metaVendorExtensibleSetter(swspec)), true),
	}
	return sp
}

type setMetaSingle struct {
	spec *spec.Swagger
	rx   *regexp.Regexp
	set  func(spec *spec.Swagger, lines []string) error
}

func (s *setMetaSingle) Matches(line string) bool {
	return s.rx.MatchString(line)
}

func (s *setMetaSingle) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := s.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		return s.set(s.spec, []string{matches[1]})
	}
	return nil
}

func setSwaggerHost(swspec *spec.Swagger, lines []string) error {
	lns := lines
	if len(lns) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		lns = []string{"localhost"}
	}
	swspec.Host = lns[0]
	return nil
}

func setSwaggerBasePath(swspec *spec.Swagger, lines []string) error {
	var ln string
	if len(lines) > 0 {
		ln = lines[0]
	}
	swspec.BasePath = ln
	return nil
}

func setInfoVersion(swspec *spec.Swagger, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	info := safeInfo(swspec)
	info.Version = strings.TrimSpace(lines[0])
	return nil
}

func setInfoContact(swspec *spec.Swagger, lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	contact, err := parseContactInfo(lines[0])
	if err != nil {
		return err
	}
	info := safeInfo(swspec)
	info.Contact = contact
	return nil
}

func parseContactInfo(line string) (*spec.ContactInfo, error) {
	nameEmail, url := splitURL(line)
	var name, email string
	if len(nameEmail) > 0 {
		addr, err := mail.ParseAddress(nameEmail)
		if err != nil {
			return nil, err
		}
		name, email = addr.Name, addr.Address
	}
	return &spec.ContactInfo{
		URL:   url,
		Name:  name,
		Email: email,
	}, nil
}

func setInfoLicense(swspec *spec.Swagger, lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	info := safeInfo(swspec)
	line := lines[0]
	name, url := splitURL(line)
	info.License = &spec.License{
		Name: name,
		URL:  url,
	}
	return nil
}

func safeInfo(swspec *spec.Swagger) *spec.Info {
	if swspec.Info == nil {
		swspec.Info = new(spec.Info)
	}
	return swspec.Info
}

// httpFTPScheme matches http://, https://, ws://, wss://
var httpFTPScheme = regexp.MustCompile("(?:(?:ht|f)tp|ws)s?://")

func splitURL(line string) (notURL, url string) {
	str := strings.TrimSpace(line)
	parts := httpFTPScheme.FindStringIndex(str)
	if len(parts) == 0 {
		if len(str) > 0 {
			notURL = str
		}
		return
	}
	if len(parts) > 0 {
		notURL = strings.TrimSpace(str[:parts[0]])
		url = strings.TrimSpace(str[parts[0]:])
	}
	return
}
