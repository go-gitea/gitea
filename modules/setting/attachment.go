// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Attachment settings
var Attachment = struct {
	Storage      *Storage
	AllowedTypes string
	MaxSize      int64
	MaxFiles     int
	Enabled      bool
}{
	Storage:      &Storage{},
	AllowedTypes: ".csv,.docx,.fodg,.fodp,.fods,.fodt,.gif,.gz,.jpeg,.jpg,.log,.md,.mov,.mp4,.odf,.odg,.odp,.ods,.odt,.patch,.pdf,.png,.pptx,.svg,.tgz,.txt,.webm,.xls,.xlsx,.zip",
	MaxSize:      4,
	MaxFiles:     5,
	Enabled:      true,
}

func loadAttachmentFrom(rootCfg ConfigProvider) (err error) {
	sec, _ := rootCfg.GetSection("attachment")
	if sec == nil {
		Attachment.Storage, err = getStorage(rootCfg, "attachments", "", nil)
		return err
	}

	Attachment.AllowedTypes = sec.Key("ALLOWED_TYPES").MustString(".csv,.docx,.fodg,.fodp,.fods,.fodt,.gif,.gz,.jpeg,.jpg,.log,.md,.mov,.mp4,.odf,.odg,.odp,.ods,.odt,.patch,.pdf,.png,.pptx,.svg,.tgz,.txt,.webm,.xls,.xlsx,.zip")
	Attachment.MaxSize = sec.Key("MAX_SIZE").MustInt64(4)
	Attachment.MaxFiles = sec.Key("MAX_FILES").MustInt(5)
	Attachment.Enabled = sec.Key("ENABLED").MustBool(true)

	Attachment.Storage, err = getStorage(rootCfg, "attachments", "", sec)
	return err
}
