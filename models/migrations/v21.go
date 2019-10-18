// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
	"xorm.io/xorm"
)

const (
	tplCommentPrefix = `# gitea public key`
	tplPublicKey     = tplCommentPrefix + "\n" + `command="%s serv key-%d --config='%s'",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty %s` + "\n"
)

func useNewPublickeyFormat(x *xorm.Engine) error {
	fpath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	if !com.IsExist(fpath) {
		return nil
	}

	tmpPath := fpath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(tmpPath)
	}()

	type PublicKey struct {
		ID      int64
		Content string
	}

	err = x.Iterate(new(PublicKey), func(idx int, bean interface{}) (err error) {
		key := bean.(*PublicKey)
		_, err = f.WriteString(fmt.Sprintf(tplPublicKey, setting.AppPath, key.ID, setting.CustomConf, key.Content))
		return err
	})
	if err != nil {
		return err
	}

	f.Close()
	return os.Rename(tmpPath, fpath)
}
