package testdata

import "embed"

var (
	//go:embed webhook
	Webhook embed.FS
)
