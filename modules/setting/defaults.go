package setting

import (
	"strings"
)

var (
	defaultLangs                             = strings.Split("en-US,zh-CN,zh-HK,zh-TW,de-DE,fr-FR,nl-NL,lv-LV,ru-RU,uk-UA,ja-JP,es-ES,pt-BR,pl-PL,bg-BG,it-IT,fi-FI,tr-TR,cs-CZ,sr-SP,sv-SE,ko-KR", ",")
	defaultLangNames                         = strings.Split("English,简体中文,繁體中文（香港）,繁體中文（台灣）,Deutsch,français,Nederlands,latviešu,русский,Українська,日本語,español,português do Brasil,polski,български,italiano,suomi,Türkçe,čeština,српски,svenska,한국어", ",")
	defaultPullRequestWorkInProgressPrefixes = strings.Split("WIP:,[WIP]", ",")
)
