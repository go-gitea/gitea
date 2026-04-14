// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package svg

import (
	"fmt"
	"html/template"
	"strings"
	"sync"

	gitea_html "code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/public"
)

type svgIconItem struct {
	html    string
	mocking bool
}

type svgCacheKey struct {
	icon  string
	size  int
	class string
}

var (
	svgIcons sync.Map // icon name -> svgIconItem; empty html means negative cache

	svgCacheMu    sync.Mutex
	svgCache      sync.Map
	svgCacheCount int
	svgCacheLimit = 10000
)

const (
	defaultSize   = 16
	svgAssetsPath = "assets/img/svg"
)

// Init is a no-op kept for compatibility. SVG icons are loaded lazily on first
// render so the backend can start before the SVG assets have been generated
// (e.g. during `make watch` or when running the binary directly from an IDE).
func Init() error {
	return nil
}

func loadIcon(icon string) svgIconItem {
	if v, ok := svgIcons.Load(icon); ok {
		return v.(svgIconItem)
	}
	bs, err := public.AssetFS().ReadFile(svgAssetsPath, icon+".svg")
	if err != nil {
		// cache the miss so repeated lookups don't keep hitting the filesystem
		svgIcons.Store(icon, svgIconItem{})
		return svgIconItem{}
	}
	item := svgIconItem{html: string(Normalize(bs, defaultSize))}
	svgIcons.Store(icon, item)
	return item
}

func MockIcon(icon string) func() {
	orig, existed := svgIcons.Load(icon)
	svgIcons.Store(icon, svgIconItem{
		html:    fmt.Sprintf(`<svg class="svg %s" width="%d" height="%d"></svg>`, icon, defaultSize, defaultSize),
		mocking: true,
	})
	return func() {
		if existed {
			svgIcons.Store(icon, orig)
		} else {
			svgIcons.Delete(icon)
		}
	}
}

// RenderHTML renders icons - arguments icon name (string), size (int), class (string)
func RenderHTML(icon string, others ...any) template.HTML {
	result, _ := renderHTML(icon, others...)
	return result
}

func renderHTML(icon string, others ...any) (_ template.HTML, usingCache bool) {
	if icon == "" {
		return "", false
	}
	size, class := gitea_html.ParseSizeAndClass(defaultSize, "", others...)
	if svgItem := loadIcon(icon); svgItem.html != "" {
		svgStr := svgItem.html
		// fast path for default size and no classes
		if size == defaultSize && class == "" {
			return template.HTML(svgStr), false
		}

		cacheKey := svgCacheKey{icon, size, class}
		cachedHTML, cached := svgCache.Load(cacheKey)
		if cached && !svgItem.mocking {
			return cachedHTML.(template.HTML), true
		}

		// the code is somewhat hacky, but it just works, because the SVG contents are all normalized
		if size != defaultSize {
			svgStr = strings.Replace(svgStr, fmt.Sprintf(`width="%d"`, defaultSize), fmt.Sprintf(`width="%d"`, size), 1)
			svgStr = strings.Replace(svgStr, fmt.Sprintf(`height="%d"`, defaultSize), fmt.Sprintf(`height="%d"`, size), 1)
		}
		if class != "" {
			svgStr = strings.Replace(svgStr, `class="`, fmt.Sprintf(`class="%s `, class), 1)
		}
		result := template.HTML(svgStr)

		if !svgItem.mocking {
			// no need to double-check, the rendering is fast enough and the cache is just an optimization
			svgCacheMu.Lock()
			if svgCacheCount >= svgCacheLimit {
				svgCache.Clear()
				svgCacheCount = 0
			}
			svgCacheCount++
			svgCache.Store(cacheKey, result)
			svgCacheMu.Unlock()
		}

		return result, false
	}

	// during test (or something wrong happens), there is no SVG loaded, so use a dummy span to tell that the icon is missing
	dummy := template.HTML(fmt.Sprintf("<span>%s(%d/%s)</span>", template.HTMLEscapeString(icon), size, template.HTMLEscapeString(class)))
	return dummy, false
}
