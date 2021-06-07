// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

func checkLabels(logger log.Logger, autofix bool) error {
	lablesCnt, issueLabelsCnt, err := models.CountWrongLabels()
	if err != nil {
		logger.Critical("Error: %v whilst counting wrong labels and issue labels")
		return err
	}
	var count int64
	if issueLabelsCnt > 0 { // We should check issue_label before label
		if autofix {
			if count, err = models.FixWrongIssueLables(); err != nil {
				logger.Critical("Error: %v whilst fixing wrong issue labels")
				return err
			}
			logger.Info("%d issue labels missing repositories fixed", count)
		} else {
			logger.Warn("%d issue labels missing repositories exist", issueLabelsCnt)
		}
	}
	if lablesCnt > 0 {
		if autofix {
			if count, err = models.FixWrongLabels(); err != nil {
				logger.Critical("Error: %v whilst fixing wrong labels")
				return err
			}
			logger.Info("%d labels missing repositories fixed", count)
		} else {
			logger.Warn("%d labels missing repositories exist", lablesCnt)
		}
	}
	return nil
}

func init() {
	Register(&Check{
		Title:                      "Check wrong labels and issue_labels where repositories deleted",
		Name:                       "labels",
		IsDefault:                  true,
		Run:                        checkLabels,
		AbortIfFailed:              true,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})
}
