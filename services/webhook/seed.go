// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"io/fs"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/options"

	"go.yaml.in/yaml/v4"
)

type webhookMetadataFile struct {
	Name                   string      `yaml:"name"`
	DisplayName            string      `yaml:"display_name"`
	DocsURL                string      `yaml:"docs_url"`
	IconAsset              string      `yaml:"icon_asset"`
	RequiresPayloadURL     *bool       `yaml:"requires_payload_url"`
	UseAuthorizationHeader string      `yaml:"use_authorization_header"`
	UseRequestSecret       string      `yaml:"use_request_secret"`
	Form                   []FormField `yaml:"form"`
}

// SeedBuiltinHookTypes loads options/webhooks/*/metadata.yml into the hook_type table (insert-if-missing).
func SeedBuiltinHookTypes(ctx context.Context) error {
	afs := options.AssetFS()
	dirs, err := afs.ListFiles("webhooks", false)
	if err != nil {
		log.Debug("SeedBuiltinHookTypes: list webhooks: %v", err)
		return nil
	}
	for _, name := range dirs {
		if err := seedOneBuiltin(ctx, afs, name); err != nil {
			return fmt.Errorf("seed webhook type %s: %w", name, err)
		}
	}
	return nil
}

type layeredReader interface {
	ReadFile(elems ...string) ([]byte, error)
}

func seedOneBuiltin(ctx context.Context, afs layeredReader, name string) error {
	metaBytes, err := afs.ReadFile("webhooks", name, "metadata.yml")
	if err != nil {
		return err
	}
	var meta webhookMetadataFile
	if err := yaml.Unmarshal(metaBytes, &meta); err != nil {
		return fmt.Errorf("parse metadata.yml: %w", err)
	}
	if meta.Name == "" {
		meta.Name = name
	}
	if meta.DisplayName == "" {
		meta.DisplayName = meta.Name
	}

	formJSON := "[]"
	if len(meta.Form) > 0 {
		b, err := json.Marshal(meta.Form)
		if err != nil {
			return err
		}
		formJSON = string(b)
	}

	payload, err := afs.ReadFile("webhooks", name, "payload.jsonnet")
	if err != nil && err != fs.ErrNotExist {
		// optional file
		payload = nil
	}
	request, err := afs.ReadFile("webhooks", name, "request.jsonnet")
	if err != nil && err != fs.ErrNotExist {
		request = nil
	}

	requiresURL := true
	if meta.RequiresPayloadURL != nil {
		requiresURL = *meta.RequiresPayloadURL
	}
	ht := &webhook_model.HookTypeDef{
		Name:                   meta.Name,
		DisplayName:            meta.DisplayName,
		DocsURL:                meta.DocsURL,
		IconAsset:              meta.IconAsset,
		FormSchema:             formJSON,
		PayloadJsonnet:         string(payload),
		RequestJsonnet:         string(request),
		RequiresPayloadURL:     requiresURL,
		UseAuthorizationHeader: meta.UseAuthorizationHeader,
		UseRequestSecret:       meta.UseRequestSecret,
		IsBuiltin:              true,
		IsActive:               true,
	}
	return webhook_model.InsertHookTypeIfNotExists(ctx, ht)
}

// LoadHandlersFromDB registers JsonnetHandlers for active hook types that define jsonnet,
// and unregisters inactive DB-defined types (including deactivated builtins).
func LoadHandlersFromDB(ctx context.Context) error {
	if err := SeedBuiltinHookTypes(ctx); err != nil {
		return err
	}
	all, err := webhook_model.ListAllHookTypes(ctx)
	if err != nil {
		log.Debug("LoadHandlersFromDB: %v", err)
		return nil
	}
	for _, ht := range all {
		if !ht.IsActive {
			UnregisterHandler(ht.Name)
			continue
		}
		if ht.PayloadJsonnet == "" && ht.RequestJsonnet == "" && ht.IsBuiltin {
			continue // keep Go handler from RegisterBuiltinHandlers
		}
		RegisterHandler(JsonnetHandler{Def: ht})
	}
	SyncWebhookTypesSetting()
	return nil
}

// ReloadHookTypeHandler (re)registers a single type after admin create/edit.
func ReloadHookTypeHandler(ht *webhook_model.HookTypeDef) {
	if !ht.IsActive {
		UnregisterHandler(ht.Name)
		SyncWebhookTypesSetting()
		return
	}
	if ht.PayloadJsonnet != "" || ht.RequestJsonnet != "" || !ht.IsBuiltin {
		RegisterHandler(JsonnetHandler{Def: ht})
	}
	SyncWebhookTypesSetting()
}

// ResetBuiltinHookType overwrites a builtin type from options/webhooks.
func ResetBuiltinHookType(ctx context.Context, name string) error {
	afs := options.AssetFS()
	metaBytes, err := afs.ReadFile("webhooks", name, "metadata.yml")
	if err != nil {
		return err
	}
	var meta webhookMetadataFile
	if err := yaml.Unmarshal(metaBytes, &meta); err != nil {
		return err
	}
	if meta.Name == "" {
		meta.Name = name
	}
	formJSON := "[]"
	if len(meta.Form) > 0 {
		b, err := json.Marshal(meta.Form)
		if err != nil {
			return err
		}
		formJSON = string(b)
	}
	payload, _ := afs.ReadFile("webhooks", name, "payload.jsonnet")
	request, _ := afs.ReadFile("webhooks", name, "request.jsonnet")
	requiresURL := true
	if meta.RequiresPayloadURL != nil {
		requiresURL = *meta.RequiresPayloadURL
	}
	ht := &webhook_model.HookTypeDef{
		Name:                   meta.Name,
		DisplayName:            meta.DisplayName,
		DocsURL:                meta.DocsURL,
		IconAsset:              meta.IconAsset,
		FormSchema:             formJSON,
		PayloadJsonnet:         string(payload),
		RequestJsonnet:         string(request),
		RequiresPayloadURL:     requiresURL,
		UseAuthorizationHeader: meta.UseAuthorizationHeader,
		UseRequestSecret:       meta.UseRequestSecret,
		IsBuiltin:              true,
		IsActive:               true,
	}
	if err := webhook_model.UpsertHookTypeByName(ctx, ht); err != nil {
		return err
	}
	ReloadHookTypeHandler(ht)
	return nil
}
