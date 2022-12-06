// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"os"

	"gopkg.in/ini.v1"
)

// enumerates all the configuration names creating
const (
	Filepath  = "custom/conf/secretKey.json"
	SecAWS    = "AWS"
	KeyAWS    = "JWK_AWS_URL"
	KeyClient = "CLIENT_PROVIDER"
)

// CheckCognitoSecretFile - check the file and return error if the file not exist
func CheckCognitoSecretFile() (os.FileInfo, error) {
	// try to open the file
	fileData, err := os.Stat(Filepath)
	return fileData, err
}

// OpenCognitoSecretFile - open the secret file
func OpenCognitoSecretFile() (*os.File, error) {
	return os.Open(Filepath)
}

// WriteInCognitoSecretFile - add key in secret file
func WriteInCognitoSecretFile(file []byte) error {
	return os.WriteFile(Filepath, file, 0644)
}

// GetAWSKey - return the key from config file
func GetAWSKey(secName string, keyName string) (*ini.Key, error) {
	return Cfg.Section(secName).GetKey(keyName)
}
