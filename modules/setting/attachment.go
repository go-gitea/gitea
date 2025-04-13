// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

type AttachmentSettingType struct {
	Storage      *Storage
	AllowedTypes string
	MaxSize      int64
	MaxFiles     int
	Enabled      bool
}

var Attachment AttachmentSettingType

func loadAttachmentFrom(rootCfg ConfigProvider) (err error) {
	Attachment = AttachmentSettingType{
		AllowedTypes: ".avif,.cpuprofile,.csv,.dmp,.docx,.fodg,.fodp,.fods,.fodt,.gif,.gz,.jpeg,.jpg,.json,.jsonc,.log,.md,.mov,.mp4,.odf,.odg,.odp,.ods,.odt,.patch,.pdf,.png,.pptx,.svg,.tgz,.txt,.webm,.webp,.xls,.xlsx,.zip",
		MaxSize:      2048,
		MaxFiles:     5,
		Enabled:      true,
	}
	sec, _ := rootCfg.GetSection("attachment")
	if sec == nil {
		Attachment.Storage, err = getStorage(rootCfg, "attachments", "", nil)
		return err
	}

	Attachment.AllowedTypes = sec.Key("ALLOWED_TYPES").MustString(Attachment.AllowedTypes)
	Attachment.MaxSize = sec.Key("MAX_SIZE").MustInt64(Attachment.MaxSize)
	Attachment.MaxFiles = sec.Key("MAX_FILES").MustInt(Attachment.MaxFiles)
	Attachment.Enabled = sec.Key("ENABLED").MustBool(Attachment.Enabled)
	Attachment.Storage, err = getStorage(rootCfg, "attachments", "", sec)
	return err
}
