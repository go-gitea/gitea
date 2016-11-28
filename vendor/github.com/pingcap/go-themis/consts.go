package themis

import "strings"

var (
	PutFamilyName  = []byte("#p")
	DelFamilyName  = []byte("#d")
	LockFamilyName = []byte("L")
)

const (
	ThemisServiceName string = "ThemisService"
)

func isWrongRegionErr(err error) bool {
	if err != nil {
		return strings.Contains(err.Error(), "org.apache.hadoop.hbase.regionserver.WrongRegionException")
	}
	return false
}
