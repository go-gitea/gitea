//go:generate go run main.go ../../

package main

import (
	"log"
	"os"
	"path/filepath"
	"regexp"

	"code.gitea.io/gitea/modules/json"
)

var rxPath = regexp.MustCompile(`(?m)^(/repos/\{owner})/(\{repo})`)

func generatePaths(root string) map[string]any {
	pathData := make(map[string]any)
	endpoints := make(map[string]any)
	fileToRead, err := filepath.Rel(root, "./templates/swagger/v1_json.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	swaggerBytes, err := os.ReadFile(fileToRead)
	if err != nil {
		log.Fatal(err)
	}
	raw := make(map[string]any)
	err = json.Unmarshal(swaggerBytes, &raw)
	if err != nil {
		log.Fatal(err)
	}
	paths := raw["paths"].(map[string]any)
	for k, v := range paths {
		if !rxPath.MatchString(k) {
			// skip if this endpoint does not start with `/repos/{owner}/{repo}`
			continue
		}
		// generate new endpoint path with `/group/{group_id}` in between the `owner` and `repo` params
		nk := rxPath.ReplaceAllString(k, "$1/group/{group_id}/$2")
		methodMap := v.(map[string]any)

		for method, methodSpec := range methodMap {
			specMap := methodSpec.(map[string]any)
			params := specMap["parameters"].([]any)
			params = append(params, map[string]any{
				"description": "group ID of the repo",
				"name":        "group_id",
				"type":        "integer",
				"format":      "int64",
				"required":    true,
				"in":          "path",
			})
			// i believe for...range loops create copies of each item that's iterated over,
			// so we need to take extra care to ensure we're mutating the original map entry
			(methodMap[method].(map[string]any))["parameters"] = params
		}
		endpoints[nk] = methodMap
	}
	pathData["paths"] = endpoints
	return pathData
}

func writeMapToFile(filename string, data map[string]any) {
	bytes, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(filename, bytes, 0o666)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	var err error
	root := "../../"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	err = os.Chdir(root)
	if err != nil {
		log.Fatal(err)
	}

	pathData := generatePaths(".")
	out := "./templates/swagger/v1_groups.json"
	writeMapToFile(out, pathData)
}
