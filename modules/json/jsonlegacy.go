//go:build !goexperiment.jsonv2

package json

import jsoniter "github.com/json-iterator/go"

func getDefaultJSONHandler() Interface {
	return JSONiter{jsoniter.ConfigCompatibleWithStandardLibrary}
}
