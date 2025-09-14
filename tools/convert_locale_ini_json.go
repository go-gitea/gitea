package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

func convertIniToJSON(data []byte) ([]byte, error) {
	iniFile, err := setting.NewConfigProviderForLocale(data)
	if err != nil {
		return nil, err
	}
	var buf strings.Builder
	buf.WriteString("{\n")
	for i, section := range iniFile.Sections() {
		isDefault := section.Name() == "" || section.Name() == "DEFAULT"
		if !isDefault {
			buf.WriteString("  \"" + section.Name() + "\": {\n")
		}
		for j, key := range section.Keys() {
			keyName := key.Name()
			if isDefault { // rename conflicted keys
				switch keyName {
				case "home":
					keyName = "_home"
				case "explore":
					keyName = "_explore"
				case "settings":
					keyName = "_settings"
				case "error":
					keyName = "_error"
				case "filter":
					keyName = "_filter"
				}
			}
			v := key.Value()
			// trim quotes
			if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
				v = v[1 : len(v)-1]
			} else {
				if strings.HasPrefix(v, "`") && strings.HasSuffix(v, "`") {
					v = v[1 : len(v)-1]
				}
				v = strings.ReplaceAll(v, `\`, `\\`)
				v = strings.ReplaceAll(v, `"`, `\"`)
			}

			if !isDefault {
				buf.WriteString("  ")
			}
			buf.WriteString(fmt.Sprintf("  \"%s\": \"%s\"", keyName, v))
			if j != len(section.Keys())-1 || isDefault {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		if !isDefault {
			buf.WriteString("  }")
			if i != len(iniFile.Sections())-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
	}
	buf.WriteString("}\n")
	return []byte(buf.String()), nil
}

func main() {
	entries, err := os.ReadDir("./options/locale")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if path.Ext(entry.Name()) != ".ini" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("./options/locale", entry.Name()))
		if err != nil {
			panic(err)
		}
		// Convert INI to JSON
		jsonData, err := convertIniToJSON(data)
		if err != nil {
			panic(err)
		}
		// Write JSON to file
		err = os.WriteFile(filepath.Join("./options/locale", entry.Name()[:len(entry.Name())-4]+".json"), jsonData, 0o644)
		if err != nil {
			panic(err)
		}
	}
}
