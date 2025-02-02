// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"encoding/xml"
	"strings"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"
)

type AtomTitle struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

type ServiceCollection struct {
	Href  string    `xml:"href,attr"`
	Title AtomTitle `xml:"atom:title"`
}

type ServiceWorkspace struct {
	Title      AtomTitle         `xml:"atom:title"`
	Collection ServiceCollection `xml:"collection"`
}

type ServiceIndexResponseV2 struct {
	XMLName   xml.Name         `xml:"service"`
	Base      string           `xml:"base,attr"`
	Xmlns     string           `xml:"xmlns,attr"`
	XmlnsAtom string           `xml:"xmlns:atom,attr"`
	Workspace ServiceWorkspace `xml:"workspace"`
}

type EdmxPropertyRef struct {
	Name string `xml:"Name,attr"`
}

type EdmxProperty struct {
	Name     string `xml:"Name,attr"`
	Type     string `xml:"Type,attr"`
	Nullable bool   `xml:"Nullable,attr"`
}

type EdmxEntityType struct {
	Name       string            `xml:"Name,attr"`
	HasStream  bool              `xml:"m:HasStream,attr"`
	Keys       []EdmxPropertyRef `xml:"Key>PropertyRef"`
	Properties []EdmxProperty    `xml:"Property"`
}

type EdmxFunctionParameter struct {
	Name string `xml:"Name,attr"`
	Type string `xml:"Type,attr"`
}

type EdmxFunctionImport struct {
	Name       string                  `xml:"Name,attr"`
	ReturnType string                  `xml:"ReturnType,attr"`
	EntitySet  string                  `xml:"EntitySet,attr"`
	Parameter  []EdmxFunctionParameter `xml:"Parameter"`
}

type EdmxEntitySet struct {
	Name       string `xml:"Name,attr"`
	EntityType string `xml:"EntityType,attr"`
}

type EdmxEntityContainer struct {
	Name                     string               `xml:"Name,attr"`
	IsDefaultEntityContainer bool                 `xml:"m:IsDefaultEntityContainer,attr"`
	EntitySet                EdmxEntitySet        `xml:"EntitySet"`
	FunctionImports          []EdmxFunctionImport `xml:"FunctionImport"`
}

type EdmxSchema struct {
	Xmlns           string               `xml:"xmlns,attr"`
	Namespace       string               `xml:"Namespace,attr"`
	EntityType      *EdmxEntityType      `xml:"EntityType,omitempty"`
	EntityContainer *EdmxEntityContainer `xml:"EntityContainer,omitempty"`
}

type EdmxDataServices struct {
	XmlnsM                string       `xml:"xmlns:m,attr"`
	DataServiceVersion    string       `xml:"m:DataServiceVersion,attr"`
	MaxDataServiceVersion string       `xml:"m:MaxDataServiceVersion,attr"`
	Schema                []EdmxSchema `xml:"Schema"`
}

type EdmxMetadata struct {
	XMLName      xml.Name         `xml:"edmx:Edmx"`
	XmlnsEdmx    string           `xml:"xmlns:edmx,attr"`
	Version      string           `xml:"Version,attr"`
	DataServices EdmxDataServices `xml:"edmx:DataServices"`
}

var Metadata = &EdmxMetadata{
	XmlnsEdmx: "http://schemas.microsoft.com/ado/2007/06/edmx",
	Version:   "1.0",
	DataServices: EdmxDataServices{
		XmlnsM:                "http://schemas.microsoft.com/ado/2007/08/dataservices/metadata",
		DataServiceVersion:    "2.0",
		MaxDataServiceVersion: "2.0",
		Schema: []EdmxSchema{
			{
				Xmlns:     "http://schemas.microsoft.com/ado/2006/04/edm",
				Namespace: "NuGetGallery.OData",
				EntityType: &EdmxEntityType{
					Name:      "V2FeedPackage",
					HasStream: true,
					Keys: []EdmxPropertyRef{
						{Name: "Id"},
						{Name: "Version"},
					},
					Properties: []EdmxProperty{
						{
							Name: "Id",
							Type: "Edm.String",
						},
						{
							Name: "Version",
							Type: "Edm.String",
						},
						{
							Name:     "NormalizedVersion",
							Type:     "Edm.String",
							Nullable: true,
						},
						{
							Name:     "Authors",
							Type:     "Edm.String",
							Nullable: true,
						},
						{
							Name: "Created",
							Type: "Edm.DateTime",
						},
						{
							Name: "Dependencies",
							Type: "Edm.String",
						},
						{
							Name: "Description",
							Type: "Edm.String",
						},
						{
							Name: "DownloadCount",
							Type: "Edm.Int64",
						},
						{
							Name: "LastUpdated",
							Type: "Edm.DateTime",
						},
						{
							Name: "Published",
							Type: "Edm.DateTime",
						},
						{
							Name: "PackageSize",
							Type: "Edm.Int64",
						},
						{
							Name:     "ProjectUrl",
							Type:     "Edm.String",
							Nullable: true,
						},
						{
							Name:     "ReleaseNotes",
							Type:     "Edm.String",
							Nullable: true,
						},
						{
							Name:     "RequireLicenseAcceptance",
							Type:     "Edm.Boolean",
							Nullable: false,
						},
						{
							Name:     "Title",
							Type:     "Edm.String",
							Nullable: true,
						},
						{
							Name:     "VersionDownloadCount",
							Type:     "Edm.Int64",
							Nullable: false,
						},
					},
				},
			},
			{
				Xmlns:     "http://schemas.microsoft.com/ado/2006/04/edm",
				Namespace: "NuGetGallery",
				EntityContainer: &EdmxEntityContainer{
					Name:                     "V2FeedContext",
					IsDefaultEntityContainer: true,
					EntitySet: EdmxEntitySet{
						Name:       "Packages",
						EntityType: "NuGetGallery.OData.V2FeedPackage",
					},
					FunctionImports: []EdmxFunctionImport{
						{
							Name:       "Search",
							ReturnType: "Collection(NuGetGallery.OData.V2FeedPackage)",
							EntitySet:  "Packages",
							Parameter: []EdmxFunctionParameter{
								{
									Name: "searchTerm",
									Type: "Edm.String",
								},
							},
						},
						{
							Name:       "FindPackagesById",
							ReturnType: "Collection(NuGetGallery.OData.V2FeedPackage)",
							EntitySet:  "Packages",
							Parameter: []EdmxFunctionParameter{
								{
									Name: "id",
									Type: "Edm.String",
								},
							},
						},
					},
				},
			},
		},
	},
}

type FeedEntryCategory struct {
	Term   string `xml:"term,attr"`
	Scheme string `xml:"scheme,attr"`
}

type FeedEntryLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type TypedValue[T any] struct {
	Type  string `xml:"type,attr,omitempty"`
	Value T      `xml:",chardata"`
}

type FeedEntryProperties struct {
	Version                  string                `xml:"d:Version"`
	NormalizedVersion        string                `xml:"d:NormalizedVersion"`
	Authors                  string                `xml:"d:Authors"`
	Dependencies             string                `xml:"d:Dependencies"`
	Description              string                `xml:"d:Description"`
	VersionDownloadCount     TypedValue[int64]     `xml:"d:VersionDownloadCount"`
	DownloadCount            TypedValue[int64]     `xml:"d:DownloadCount"`
	PackageSize              TypedValue[int64]     `xml:"d:PackageSize"`
	Created                  TypedValue[time.Time] `xml:"d:Created"`
	LastUpdated              TypedValue[time.Time] `xml:"d:LastUpdated"`
	Published                TypedValue[time.Time] `xml:"d:Published"`
	ProjectURL               string                `xml:"d:ProjectUrl,omitempty"`
	ReleaseNotes             string                `xml:"d:ReleaseNotes,omitempty"`
	RequireLicenseAcceptance TypedValue[bool]      `xml:"d:RequireLicenseAcceptance"`
	Title                    string                `xml:"d:Title"`
}

type FeedEntry struct {
	XMLName    xml.Name             `xml:"entry"`
	Xmlns      string               `xml:"xmlns,attr,omitempty"`
	XmlnsD     string               `xml:"xmlns:d,attr,omitempty"`
	XmlnsM     string               `xml:"xmlns:m,attr,omitempty"`
	Base       string               `xml:"xml:base,attr,omitempty"`
	ID         string               `xml:"id"`
	Category   FeedEntryCategory    `xml:"category"`
	Links      []FeedEntryLink      `xml:"link"`
	Title      TypedValue[string]   `xml:"title"`
	Updated    time.Time            `xml:"updated"`
	Author     string               `xml:"author>name"`
	Summary    string               `xml:"summary"`
	Properties *FeedEntryProperties `xml:"m:properties"`
	Content    string               `xml:",innerxml"`
}

type FeedResponse struct {
	XMLName xml.Name           `xml:"feed"`
	Xmlns   string             `xml:"xmlns,attr,omitempty"`
	XmlnsD  string             `xml:"xmlns:d,attr,omitempty"`
	XmlnsM  string             `xml:"xmlns:m,attr,omitempty"`
	Base    string             `xml:"xml:base,attr,omitempty"`
	ID      string             `xml:"id"`
	Title   TypedValue[string] `xml:"title"`
	Updated time.Time          `xml:"updated"`
	Links   []FeedEntryLink    `xml:"link"`
	Entries []*FeedEntry       `xml:"entry"`
	Count   int64              `xml:"m:count"`
}

func createFeedResponse(l *linkBuilder, totalEntries int64, pds []*packages_model.PackageDescriptor) *FeedResponse {
	entries := make([]*FeedEntry, 0, len(pds))
	for _, pd := range pds {
		entries = append(entries, createEntry(l, pd, false))
	}

	links := []FeedEntryLink{
		{Rel: "self", Href: l.Base},
	}
	if l.Next != nil {
		links = append(links, FeedEntryLink{
			Rel:  "next",
			Href: l.GetNextURL(),
		})
	}

	return &FeedResponse{
		Xmlns:   "http://www.w3.org/2005/Atom",
		Base:    l.Base,
		XmlnsD:  "http://schemas.microsoft.com/ado/2007/08/dataservices",
		XmlnsM:  "http://schemas.microsoft.com/ado/2007/08/dataservices/metadata",
		ID:      "http://schemas.datacontract.org/2004/07/",
		Updated: time.Now(),
		Links:   links,
		Count:   totalEntries,
		Entries: entries,
	}
}

func createEntryResponse(l *linkBuilder, pd *packages_model.PackageDescriptor) *FeedEntry {
	return createEntry(l, pd, true)
}

func createEntry(l *linkBuilder, pd *packages_model.PackageDescriptor, withNamespace bool) *FeedEntry {
	metadata := pd.Metadata.(*nuget_module.Metadata)

	id := l.GetPackageMetadataURL(pd.Package.Name, pd.Version.Version)

	// Workaround to force a self-closing tag to satisfy XmlReader.IsEmptyElement used by the NuGet client.
	// https://learn.microsoft.com/en-us/dotnet/api/system.xml.xmlreader.isemptyelement
	content := `<content type="application/zip" src="` + l.GetPackageDownloadURL(pd.Package.Name, pd.Version.Version) + `"/>`

	createdValue := TypedValue[time.Time]{
		Type:  "Edm.DateTime",
		Value: pd.Version.CreatedUnix.AsLocalTime(),
	}

	entry := &FeedEntry{
		ID:       id,
		Category: FeedEntryCategory{Term: "NuGetGallery.OData.V2FeedPackage", Scheme: "http://schemas.microsoft.com/ado/2007/08/dataservices/scheme"},
		Links: []FeedEntryLink{
			{Rel: "self", Href: id},
			{Rel: "edit", Href: id},
		},
		Title:   TypedValue[string]{Type: "text", Value: pd.Package.Name},
		Updated: pd.Version.CreatedUnix.AsLocalTime(),
		Author:  metadata.Authors,
		Content: content,
		Properties: &FeedEntryProperties{
			Version:                  pd.Version.Version,
			NormalizedVersion:        pd.Version.Version,
			Authors:                  metadata.Authors,
			Dependencies:             buildDependencyString(metadata),
			Description:              metadata.Description,
			VersionDownloadCount:     TypedValue[int64]{Type: "Edm.Int64", Value: pd.Version.DownloadCount},
			DownloadCount:            TypedValue[int64]{Type: "Edm.Int64", Value: pd.Version.DownloadCount},
			PackageSize:              TypedValue[int64]{Type: "Edm.Int64", Value: pd.CalculateBlobSize()},
			Created:                  createdValue,
			LastUpdated:              createdValue,
			Published:                createdValue,
			ProjectURL:               metadata.ProjectURL,
			ReleaseNotes:             metadata.ReleaseNotes,
			RequireLicenseAcceptance: TypedValue[bool]{Type: "Edm.Boolean", Value: metadata.RequireLicenseAcceptance},
			Title:                    pd.Package.Name,
		},
	}

	if withNamespace {
		entry.Xmlns = "http://www.w3.org/2005/Atom"
		entry.Base = l.Base
		entry.XmlnsD = "http://schemas.microsoft.com/ado/2007/08/dataservices"
		entry.XmlnsM = "http://schemas.microsoft.com/ado/2007/08/dataservices/metadata"
	}

	return entry
}

func buildDependencyString(metadata *nuget_module.Metadata) string {
	var b strings.Builder
	first := true
	for group, deps := range metadata.Dependencies {
		for _, dep := range deps {
			if !first {
				b.WriteByte('|')
			}
			first = false

			b.WriteString(dep.ID)
			b.WriteByte(':')
			b.WriteString(dep.Version)
			b.WriteByte(':')
			b.WriteString(group)
		}
	}
	return b.String()
}
