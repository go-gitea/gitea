// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/olivere/elastic/v7"
)

// IndexName returns the full index name with version
func (i *Indexer) IndexName() string {
	return fmt.Sprintf("%s.v%d", i.indexAliasName, i.version)
}

func (i *Indexer) createIndex(ctx context.Context) error {
	createIndex, err := i.Client.CreateIndex(i.IndexName()).BodyString(i.mapping).Do(ctx)
	if err != nil {
		return err
	}
	if !createIndex.Acknowledged {
		return fmt.Errorf("create index %s with %s failed", i.IndexName(), i.mapping)
	}

	// check version
	r, err := i.Client.Aliases().Do(ctx)
	if err != nil {
		return err
	}

	realIndexerNames := r.IndicesByAlias(i.indexAliasName)
	if len(realIndexerNames) < 1 {
		res, err := i.Client.Alias().
			Add(i.IndexName(), i.indexAliasName).
			Do(ctx)
		if err != nil {
			return err
		}
		if !res.Acknowledged {
			return fmt.Errorf("create alias %s to index %s failed", i.indexAliasName, i.IndexName())
		}
	} else if len(realIndexerNames) >= 1 && realIndexerNames[0] < i.IndexName() {
		log.Warn("Found older gitea indexer named %s, but we will create a new one %s and keep the old NOT DELETED. You can delete the old version after the upgrade succeed.",
			realIndexerNames[0], i.IndexName())
		res, err := i.Client.Alias().
			Remove(realIndexerNames[0], i.indexAliasName).
			Add(i.IndexName(), i.indexAliasName).
			Do(ctx)
		if err != nil {
			return err
		}
		if !res.Acknowledged {
			return fmt.Errorf("change alias %s to index %s failed", i.indexAliasName, i.IndexName())
		}
	}

	return nil
}

func (i *Indexer) initClient() (*elastic.Client, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(i.url),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10 * time.Second),
		elastic.SetGzip(false),
	}

	logger := log.GetLogger(log.DEFAULT)

	opts = append(opts, elastic.SetTraceLog(&log.PrintfLogger{Logf: logger.Trace}))
	opts = append(opts, elastic.SetInfoLog(&log.PrintfLogger{Logf: logger.Info}))
	opts = append(opts, elastic.SetErrorLog(&log.PrintfLogger{Logf: logger.Error}))

	return elastic.NewClient(opts...)
}
