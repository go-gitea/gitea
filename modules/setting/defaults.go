package setting

import (
	"strings"
)

var (
	defaultLangs     = strings.Split("en-US,zh-CN,zh-HK,zh-TW,de-DE,fr-FR,nl-NL,lv-LV,ru-RU,ja-JP,es-ES,pt-BR,pl-PL,bg-BG,it-IT,fi-FI,tr-TR,cs-CZ,sr-SP,sv-SE,ko-KR", ",")
	defaultLangNames = strings.Split("English,简体中文,繁體中文（香港）,繁體中文（台灣）,Deutsch,Français,Nederlands,Latviešu,Русский,日本語,Español,Português do Brasil,Polski,български,Italiano,Suomalainen,Türkçe,čeština,Српски,Svenska,한국어", ",")
)
