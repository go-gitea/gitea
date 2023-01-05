// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This artifact server is inspired by https://github.com/nektos/act/blob/master/pkg/artifacts/server.go.
// It updates url setting and uses ObjectStore to handle artifacts persistence.

package actions

import (
	"context"
	"fmt"
	"log"
	"net"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// https://stackoverflow.com/a/37382208
// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func init() {
	db.RegisterModel(new(ActionArtifact))
}

// ActionArtifact is a file that is stored in the artifact storage.
type ActionArtifact struct {
	ID           int64
	JobID        int64
	RunnerID     int64  `xorm:"index"`
	RepoID       int64  `xorm:"index"`
	OwnerID      int64  `xorm:"index"`
	CommitSHA    string `xorm:"index"`
	FilePath     string
	FileSize     int64
	ArtifactPath string
	ArtifactName string
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated index"`
}

// CreateArtifact creates a new artifact with task info
func CreateArtifact(ctx context.Context, t *ActionTask) (*ActionArtifact, error) {
	artifact := &ActionArtifact{
		JobID:     t.JobID,
		RunnerID:  t.RunnerID,
		RepoID:    t.RepoID,
		OwnerID:   t.OwnerID,
		CommitSHA: t.CommitSHA,
	}
	_, err := db.GetEngine(ctx).Insert(artifact)
	return artifact, err
}

// GetArtifactByID returns an artifact by id
func GetArtifactByID(ctx context.Context, id int64) (*ActionArtifact, error) {
	var art ActionArtifact
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("task with id %d: %w", id, util.ErrNotExist)
	}

	return &art, nil
}

// UpdateArtifactByID updates an artifact by id
func UpdateArtifactByID(ctx context.Context, id int64, art *ActionArtifact) error {
	art.ID = id
	_, err := db.GetEngine(ctx).ID(id).AllCols().Update(art)
	return err
}

// ListArtifactByJobID returns all artifacts of a job
func ListArtifactByJobID(ctx context.Context, jobID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("job_id=?", jobID).Find(&arts)
}
