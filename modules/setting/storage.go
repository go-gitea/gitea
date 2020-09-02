// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

var (
	// Storage settings
	Storage = struct {
		StoreType   string
		ServeDirect bool
		Minio       struct {
			Endpoint        string
			AccessKeyID     string
			SecretAccessKey string
			UseSSL          bool
			Bucket          string
			Location        string
		}
	}{
		StoreType: "local",
	}
)

func newStorageService() {
	sec := Cfg.Section("storage")
	Storage.StoreType = sec.Key("STORE_TYPE").MustString("local")
	Storage.ServeDirect = sec.Key("SERVE_DIRECT").MustBool(false)
	switch Attachment.StoreType {
	case "local":
	case "minio":
		Storage.Minio.Endpoint = sec.Key("MINIO_ENDPOINT").MustString("localhost:9000")
		Storage.Minio.AccessKeyID = sec.Key("MINIO_ACCESS_KEY_ID").MustString("")
		Storage.Minio.SecretAccessKey = sec.Key("MINIO_SECRET_ACCESS_KEY").MustString("")
		Storage.Minio.Bucket = sec.Key("MINIO_BUCKET").MustString("gitea")
		Storage.Minio.Location = sec.Key("MINIO_LOCATION").MustString("us-east-1")
		Storage.Minio.UseSSL = sec.Key("MINIO_USE_SSL").MustBool(false)
	}
}
