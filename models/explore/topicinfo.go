// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/options"
)

type TopicInfo struct {
	ID               int64  `xorm:"pk autoincr"`
	Name             string `xorm:"UNIQUE(s) index"`
	DisplayName      string
	ShortDescription string
	Markdown         string
	RenderedMarkdown string `xorm:"-"`
	WebsiteUrl       string
	WikipediaUrl     string
}

type TopicIndex struct {
	Topic            string `yaml:"topic"`
	DisplayName      string `yaml:"display_name"`
	ShortDescription string `yaml:"short_description"`
	Aliases          string `yaml:"aliases"`
	WebsiteUrl       string `yaml:"url"`
	WikipediaUrl     string `yaml:"wikipedia_url"`
}

func init() {
	db.RegisterModel(new(TopicInfo))
}

func (topic *TopicInfo) RenderMarkdown(ctx context.Context) error {
	var err error
	topic.RenderedMarkdown, err = markdown.RenderString(&markup.RenderContext{Ctx: ctx}, topic.Markdown)
	return err
}

func GetTopicInfoByName(ctx context.Context, name string) (*TopicInfo, error) {
	topic := new(TopicInfo)
	has, err := db.GetEngine(ctx).Where("name = ?", strings.ToLower(name)).Get(topic)
	if err != nil {
		return nil, err
	}

	if has {
		return topic, nil
	} else {
		return nil, nil
	}
}

func UpdateTopicInfo(ctx context.Context) error {
	log.Info("Started topic")

	topicList, err := options.AssetFS().ListFiles("topics")
	if err != nil {
		return err
	}

	err = db.DeleteAllRecords("topic_info")
	if err != nil {
		return err
	}

	for _, topicDir := range topicList {
		content, err := options.AssetFS().ReadFile("topics", topicDir, "index.md")
		if err != nil {
			return err
		}

		topicData := new(TopicInfo)

		index := new(TopicIndex)
		topicData.Markdown, err = markdown.ExtractMetadata(string(content[:]), index)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		topicData.Name = index.Topic
		topicData.DisplayName = index.DisplayName
		topicData.ShortDescription = index.ShortDescription
		topicData.WebsiteUrl = index.WebsiteUrl
		topicData.WikipediaUrl = index.WikipediaUrl

		err = db.Insert(ctx, topicData)
		if err != nil {
			return err
		}

		if index.Aliases != "" {
			for _, alias := range strings.Split(index.Aliases, ",") {
				topicData.ID = 0
				topicData.Name = strings.TrimSpace(alias)
				err = db.Insert(ctx, topicData)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
