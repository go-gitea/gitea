// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	gossh "golang.org/x/crypto/ssh"
)

var SSH = struct {
	Disabled                              bool               `ini:"DISABLE_SSH"`
	StartBuiltinServer                    bool               `ini:"START_SSH_SERVER"`
	BuiltinServerUser                     string             `ini:"BUILTIN_SSH_SERVER_USER"`
	UseProxyProtocol                      bool               `ini:"SSH_SERVER_USE_PROXY_PROTOCOL"`
	Domain                                string             `ini:"SSH_DOMAIN"`
	Port                                  int                `ini:"SSH_PORT"`
	User                                  string             `ini:"SSH_USER"`
	ListenHost                            string             `ini:"SSH_LISTEN_HOST"`
	ListenPort                            int                `ini:"SSH_LISTEN_PORT"`
	RootPath                              string             `ini:"SSH_ROOT_PATH"`
	ServerCiphers                         []string           `ini:"SSH_SERVER_CIPHERS"`
	ServerKeyExchanges                    []string           `ini:"SSH_SERVER_KEY_EXCHANGES"`
	ServerMACs                            []string           `ini:"SSH_SERVER_MACS"`
	ServerHostKeys                        []string           `ini:"SSH_SERVER_HOST_KEYS"`
	KeyTestPath                           string             `ini:"SSH_KEY_TEST_PATH"`
	KeygenPath                            string             `ini:"SSH_KEYGEN_PATH"`
	AuthorizedKeysBackup                  bool               `ini:"SSH_AUTHORIZED_KEYS_BACKUP"`
	AuthorizedPrincipalsBackup            bool               `ini:"SSH_AUTHORIZED_PRINCIPALS_BACKUP"`
	AuthorizedKeysCommandTemplate         string             `ini:"SSH_AUTHORIZED_KEYS_COMMAND_TEMPLATE"`
	AuthorizedKeysCommandTemplateTemplate *template.Template `ini:"-"`
	MinimumKeySizeCheck                   bool               `ini:"-"`
	MinimumKeySizes                       map[string]int     `ini:"-"`
	CreateAuthorizedKeysFile              bool               `ini:"SSH_CREATE_AUTHORIZED_KEYS_FILE"`
	CreateAuthorizedPrincipalsFile        bool               `ini:"SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE"`
	ExposeAnonymous                       bool               `ini:"SSH_EXPOSE_ANONYMOUS"`
	AuthorizedPrincipalsAllow             []string           `ini:"SSH_AUTHORIZED_PRINCIPALS_ALLOW"`
	AuthorizedPrincipalsEnabled           bool               `ini:"-"`
	TrustedUserCAKeys                     []string           `ini:"SSH_TRUSTED_USER_CA_KEYS"`
	TrustedUserCAKeysFile                 string             `ini:"SSH_TRUSTED_USER_CA_KEYS_FILENAME"`
	TrustedUserCAKeysParsed               []gossh.PublicKey  `ini:"-"`
	PerWriteTimeout                       time.Duration      `ini:"SSH_PER_WRITE_TIMEOUT"`
	PerWritePerKbTimeout                  time.Duration      `ini:"SSH_PER_WRITE_PER_KB_TIMEOUT"`
}{
	Disabled:                      false,
	StartBuiltinServer:            false,
	Domain:                        "",
	Port:                          22,
	ServerCiphers:                 []string{"chacha20-poly1305@openssh.com", "aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com", "aes256-gcm@openssh.com"},
	ServerKeyExchanges:            []string{"curve25519-sha256", "ecdh-sha2-nistp256", "ecdh-sha2-nistp384", "ecdh-sha2-nistp521", "diffie-hellman-group14-sha256", "diffie-hellman-group14-sha1"},
	ServerMACs:                    []string{"hmac-sha2-256-etm@openssh.com", "hmac-sha2-256", "hmac-sha1"},
	KeygenPath:                    "",
	MinimumKeySizeCheck:           true,
	MinimumKeySizes:               map[string]int{"ed25519": 256, "ed25519-sk": 256, "ecdsa": 256, "ecdsa-sk": 256, "rsa": 2047},
	ServerHostKeys:                []string{"ssh/gitea.rsa", "ssh/gogs.rsa"},
	AuthorizedKeysCommandTemplate: "{{.AppPath}} --config={{.CustomConf}} serv key-{{.Key.ID}}",
	PerWriteTimeout:               PerWriteTimeout,
	PerWritePerKbTimeout:          PerWritePerKbTimeout,
}

func parseAuthorizedPrincipalsAllow(values []string) ([]string, bool) {
	anything := false
	email := false
	username := false
	for _, value := range values {
		v := strings.ToLower(strings.TrimSpace(value))
		switch v {
		case "off":
			return []string{"off"}, false
		case "email":
			email = true
		case "username":
			username = true
		case "anything":
			anything = true
		}
	}
	if anything {
		return []string{"anything"}, true
	}

	authorizedPrincipalsAllow := []string{}
	if username {
		authorizedPrincipalsAllow = append(authorizedPrincipalsAllow, "username")
	}
	if email {
		authorizedPrincipalsAllow = append(authorizedPrincipalsAllow, "email")
	}

	return authorizedPrincipalsAllow, true
}

func loadSSHFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("server")
	if len(SSH.Domain) == 0 {
		SSH.Domain = Domain
	}

	homeDir, err := util.HomeDir()
	if err != nil {
		log.Fatal("Failed to get home directory: %v", err)
	}
	homeDir = strings.ReplaceAll(homeDir, "\\", "/")

	SSH.RootPath = path.Join(homeDir, ".ssh")
	serverCiphers := sec.Key("SSH_SERVER_CIPHERS").Strings(",")
	if len(serverCiphers) > 0 {
		SSH.ServerCiphers = serverCiphers
	}
	serverKeyExchanges := sec.Key("SSH_SERVER_KEY_EXCHANGES").Strings(",")
	if len(serverKeyExchanges) > 0 {
		SSH.ServerKeyExchanges = serverKeyExchanges
	}
	serverMACs := sec.Key("SSH_SERVER_MACS").Strings(",")
	if len(serverMACs) > 0 {
		SSH.ServerMACs = serverMACs
	}
	SSH.KeyTestPath = os.TempDir()
	if err = sec.MapTo(&SSH); err != nil {
		log.Fatal("Failed to map SSH settings: %v", err)
	}
	for i, key := range SSH.ServerHostKeys {
		if !filepath.IsAbs(key) {
			SSH.ServerHostKeys[i] = filepath.Join(AppDataPath, key)
		}
	}

	SSH.KeygenPath = sec.Key("SSH_KEYGEN_PATH").String()
	SSH.Port = sec.Key("SSH_PORT").MustInt(22)
	SSH.ListenPort = sec.Key("SSH_LISTEN_PORT").MustInt(SSH.Port)
	SSH.UseProxyProtocol = sec.Key("SSH_SERVER_USE_PROXY_PROTOCOL").MustBool(false)

	// When disable SSH, start builtin server value is ignored.
	if SSH.Disabled {
		SSH.StartBuiltinServer = false
	}

	SSH.TrustedUserCAKeysFile = sec.Key("SSH_TRUSTED_USER_CA_KEYS_FILENAME").MustString(filepath.Join(SSH.RootPath, "gitea-trusted-user-ca-keys.pem"))

	for _, caKey := range SSH.TrustedUserCAKeys {
		pubKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(caKey))
		if err != nil {
			log.Fatal("Failed to parse TrustedUserCaKeys: %s %v", caKey, err)
		}

		SSH.TrustedUserCAKeysParsed = append(SSH.TrustedUserCAKeysParsed, pubKey)
	}
	if len(SSH.TrustedUserCAKeys) > 0 {
		// Set the default as email,username otherwise we can leave it empty
		sec.Key("SSH_AUTHORIZED_PRINCIPALS_ALLOW").MustString("username,email")
	} else {
		sec.Key("SSH_AUTHORIZED_PRINCIPALS_ALLOW").MustString("off")
	}

	SSH.AuthorizedPrincipalsAllow, SSH.AuthorizedPrincipalsEnabled = parseAuthorizedPrincipalsAllow(sec.Key("SSH_AUTHORIZED_PRINCIPALS_ALLOW").Strings(","))

	SSH.MinimumKeySizeCheck = sec.Key("MINIMUM_KEY_SIZE_CHECK").MustBool(SSH.MinimumKeySizeCheck)
	minimumKeySizes := rootCfg.Section("ssh.minimum_key_sizes").Keys()
	for _, key := range minimumKeySizes {
		if key.MustInt() != -1 {
			SSH.MinimumKeySizes[strings.ToLower(key.Name())] = key.MustInt()
		} else {
			delete(SSH.MinimumKeySizes, strings.ToLower(key.Name()))
		}
	}

	SSH.AuthorizedKeysBackup = sec.Key("SSH_AUTHORIZED_KEYS_BACKUP").MustBool(false)
	SSH.CreateAuthorizedKeysFile = sec.Key("SSH_CREATE_AUTHORIZED_KEYS_FILE").MustBool(true)

	SSH.AuthorizedPrincipalsBackup = false
	SSH.CreateAuthorizedPrincipalsFile = false
	if SSH.AuthorizedPrincipalsEnabled {
		SSH.AuthorizedPrincipalsBackup = sec.Key("SSH_AUTHORIZED_PRINCIPALS_BACKUP").MustBool(true)
		SSH.CreateAuthorizedPrincipalsFile = sec.Key("SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE").MustBool(true)
	}

	SSH.ExposeAnonymous = sec.Key("SSH_EXPOSE_ANONYMOUS").MustBool(false)
	SSH.AuthorizedKeysCommandTemplate = sec.Key("SSH_AUTHORIZED_KEYS_COMMAND_TEMPLATE").MustString(SSH.AuthorizedKeysCommandTemplate)

	SSH.AuthorizedKeysCommandTemplateTemplate = template.Must(template.New("").Parse(SSH.AuthorizedKeysCommandTemplate))

	SSH.PerWriteTimeout = sec.Key("SSH_PER_WRITE_TIMEOUT").MustDuration(PerWriteTimeout)
	SSH.PerWritePerKbTimeout = sec.Key("SSH_PER_WRITE_PER_KB_TIMEOUT").MustDuration(PerWritePerKbTimeout)

	// ensure parseRunModeSetting has been executed before this
	SSH.BuiltinServerUser = rootCfg.Section("server").Key("BUILTIN_SSH_SERVER_USER").MustString(RunUser)
	SSH.User = rootCfg.Section("server").Key("SSH_USER").MustString(SSH.BuiltinServerUser)
}
