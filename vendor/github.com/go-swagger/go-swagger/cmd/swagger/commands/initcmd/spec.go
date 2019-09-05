package initcmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

// Spec a command struct for initializing a new swagger application.
type Spec struct {
	Format      string   `long:"format" description:"the format for the spec document" default:"yaml" choice:"yaml" choice:"json"`
	Title       string   `long:"title" description:"the title of the API"`
	Description string   `long:"description" description:"the description of the API"`
	Version     string   `long:"version" description:"the version of the API" default:"0.1.0"`
	Terms       string   `long:"terms" description:"the terms of services"`
	Consumes    []string `long:"consumes" description:"add a content type to the global consumes definitions, can repeat" default:"application/json"`
	Produces    []string `long:"produces" description:"add a content type to the global produces definitions, can repeat" default:"application/json"`
	Schemes     []string `long:"scheme" description:"add a scheme to the global schemes definition, can repeat" default:"http"`
	Contact     struct {
		Name  string `long:"contact.name" description:"name of the primary contact for the API"`
		URL   string `long:"contact.url" description:"url of the primary contact for the API"`
		Email string `long:"contact.email" description:"email of the primary contact for the API"`
	}
	License struct {
		Name string `long:"license.name" description:"name of the license for the API"`
		URL  string `long:"license.url" description:"url of the license for the API"`
	}
}

// Execute this command
func (s *Spec) Execute(args []string) error {
	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}
	realPath, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	var file *os.File
	switch s.Format {
	case "json":
		file, err = os.Create(filepath.Join(realPath, "swagger.json"))
		if err != nil {
			return err
		}
	case "yaml", "yml":
		file, err = os.Create(filepath.Join(realPath, "swagger.yml"))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid format: %s", s.Format)
	}
	defer file.Close()
	log.Println("creating specification document in", filepath.Join(targetPath, file.Name()))

	var doc spec.Swagger
	info := new(spec.Info)
	doc.Info = info

	doc.Swagger = "2.0"
	doc.Paths = new(spec.Paths)
	doc.Definitions = make(spec.Definitions)

	info.Title = s.Title
	if info.Title == "" {
		info.Title = swag.ToHumanNameTitle(filepath.Base(realPath))
	}
	info.Description = s.Description
	info.Version = s.Version
	info.TermsOfService = s.Terms
	if s.Contact.Name != "" || s.Contact.Email != "" || s.Contact.URL != "" {
		var contact spec.ContactInfo
		contact.Name = s.Contact.Name
		contact.Email = s.Contact.Email
		contact.URL = s.Contact.URL
		info.Contact = &contact
	}
	if s.License.Name != "" || s.License.URL != "" {
		var license spec.License
		license.Name = s.License.Name
		license.URL = s.License.URL
		info.License = &license
	}

	doc.Consumes = append(doc.Consumes, s.Consumes...)
	doc.Produces = append(doc.Produces, s.Produces...)
	doc.Schemes = append(doc.Schemes, s.Schemes...)

	if s.Format == "json" {
		enc := json.NewEncoder(file)
		return enc.Encode(doc)
	}

	b, err := yaml.Marshal(swag.ToDynamicJSON(doc))
	if err != nil {
		return err
	}
	if _, err := file.Write(b); err != nil {
		return err
	}
	return nil
}
