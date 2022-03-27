// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"context"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/nosql"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/lunny/levelqueue"
)

var levelqueueTypes = []string{
	string(queue.PersistableChannelQueueType),
	string(queue.PersistableChannelUniqueQueueType),
	string(queue.LevelQueueType),
	string(queue.LevelUniqueQueueType),
}

func checkUniqueQueues(ctx context.Context, logger log.Logger, autofix bool) error {
	for _, name := range queue.KnownUniqueQueueNames {
		q := setting.GetQueueSettings(string(name))
		if q.Type == "" {
			q.Type = string(queue.PersistableChannelQueueType)
		}
		found := false
		for _, typ := range levelqueueTypes {
			if typ == q.Type {
				found = true
				break
			}
		}
		if !found {
			logger.Info("Queue: %s\nType: %s\nNo LevelDB", q.Name, q.Type)
			continue
		}

		connection := q.ConnectionString
		if connection == "" {
			connection = q.DataDir
		}

		db, err := nosql.GetManager().GetLevelDB(connection)
		if err != nil {
			logger.Error("Queue: %s\nUnable to open DB connection %q: %v", q.Name, connection, err)
			return err
		}
		defer db.Close()

		prefix := q.Name

		iQueue, err := levelqueue.NewQueue(db, []byte(prefix), false)
		if err != nil {
			logger.Error("Queue: %s\nUnable to open Queue component: %v", q.Name, err)
			return err
		}

		iSet, err := levelqueue.NewSet(db, []byte(prefix+"-unique"), false)
		if err != nil {
			logger.Error("Queue: %s\nUnable to open Set component: %v", q.Name, err)
			return err
		}

		qLen := iQueue.Len()
		sMembers, err := iSet.Members()
		if err != nil {
			logger.Error("Queue: %s\nUnable to get members of Set component: %v", q.Name, err)
			return err
		}
		sLen := len(sMembers)

		if int(qLen) == sLen {
			if qLen == 0 {
				logger.Info("Queue: %s\nType: %s\nLevelDB: %s", q.Name, q.Type, "empty")
			} else {
				logger.Info("Queue: %s\nType: %s\nLevelDB contains: %d entries", q.Name, q.Type, qLen)
			}
			continue
		}
		logger.Warn("Queue: %s\nType: %s\nContains different numbers of elements in Queue component %d to Set component %d", q.Name, q.Type, qLen, sLen)
		if !autofix {
			continue
		}

		// Empty out the old set members
		for _, member := range sMembers {
			_, err := iSet.Remove(member)
			if err != nil {
				logger.Error("Queue: %s\nUnable to remove Set member %s: %v", q.Name, string(member), err)
				return err
			}
		}

		// Now iterate across the queue
		for i := int64(0); i < qLen; i++ {
			// Pop from the left
			qData, err := iQueue.LPop()
			if err != nil {
				logger.Error("Queue: %s\nUnable to LPop out: %v", q.Name, err)
				return err
			}
			// And add to the right
			err = iQueue.RPush(qData)
			if err != nil {
				logger.Error("Queue: %s\nUnable to RPush back: %v", q.Name, err)
				return err
			}
			// And add back to the set
			_, err = iSet.Add(qData)
			if err != nil {
				logger.Error("Queue: %s\nUnable to add back in to Set: %v", q.Name, err)
				return err
			}
		}
	}
	return nil
}

func queueListDB(ctx context.Context, logger log.Logger, autofix bool) error {
	connections := []string{}
	queueNames := make([]string, 0, len(queue.KnownUniqueQueueNames)+len(queue.KnownQueueNames))
	for _, name := range queue.KnownUniqueQueueNames {
		queueNames = append(queueNames, string(name))
	}
	for _, name := range queue.KnownQueueNames {
		queueNames = append(queueNames, string(name))
	}

	for _, name := range queueNames {
		q := setting.GetQueueSettings(name)
		if q.Type == "" {
			q.Type = string(queue.PersistableChannelQueueType)
		}
		found := false
		for _, typ := range levelqueueTypes {
			if typ == q.Type {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		if q.ConnectionString != "" {
			found := false
			for _, connection := range connections {
				if connection == q.ConnectionString {
					found = true
					break
				}
			}
			if !found {
				connections = append(connections, q.ConnectionString)
			}
			continue
		}
		found = false
		for _, connection := range connections {
			if connection == q.DataDir {
				found = true
				break
			}
		}
		if !found {
			connections = append(connections, q.DataDir)
		}
	}

	for _, connection := range connections {
		logger.Info("LevelDB: %s", connection)
		db, err := nosql.GetManager().GetLevelDB(connection)
		if err != nil {
			logger.Error("Connection: %q Unable to open DB: %v", connection, err)
			return err
		}
		defer db.Close()
		iter := db.NewIterator(nil, nil)
		for iter.Next() {
			logger.Info("%s\n%s", log.NewColoredIDValue(string(iter.Key())), string(iter.Value()))
		}
		iter.Release()
	}
	return nil
}

func init() {
	Register(&Check{
		Title:                      "Check if there are corrupt level uniquequeues",
		Name:                       "uniquequeues-corrupt",
		IsDefault:                  false,
		Run:                        checkUniqueQueues,
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "List all entries in leveldb",
		Name:                       "queues-listdb",
		IsDefault:                  false,
		Run:                        queueListDB,
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})
}
