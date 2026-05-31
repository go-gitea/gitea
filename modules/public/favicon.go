// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

const (
	FaviconVariantSuccess = "success"
	FaviconVariantPending = "pending"
	FaviconVariantFailure = "failure"
)

var faviconVariants = []string{
	FaviconVariantSuccess,
	FaviconVariantPending,
	FaviconVariantFailure,
}

func isFaviconVariant(variant string) bool {
	for _, validVariant := range faviconVariants {
		if variant == validVariant {
			return true
		}
	}
	return false
}

func customAssetExists(name string) bool {
	f, err := CustomAssets().Open(name)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func customFaviconExists() bool {
	return customAssetExists("assets/img/favicon.svg") || customAssetExists("assets/img/favicon.png")
}

func customFaviconVariantsComplete() bool {
	for _, variant := range faviconVariants {
		if !customAssetExists("assets/img/favicon-"+variant+".svg") || !customAssetExists("assets/img/favicon-"+variant+".png") {
			return false
		}
	}
	return true
}

// FaviconVariantAvailable reports whether a status favicon variant can be used without mixing
// builtin status icons with a user-provided base favicon.
func FaviconVariantAvailable(variant string) bool {
	if !isFaviconVariant(variant) {
		return false
	}
	if !customFaviconExists() {
		return true
	}
	return customFaviconVariantsComplete()
}
