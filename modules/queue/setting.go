// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func validType(t string) (Type, error) {
	if len(t) == 0 {
		return PersistableChannelQueueType, nil
	}
	for _, typ := range RegisteredTypes() {
		if t == string(typ) {
			return typ, nil
		}
	}
	return PersistableChannelQueueType, fmt.Errorf("Unknown queue type: %s defaulting to %s", t, string(PersistableChannelQueueType))
}

// CreateQueue for name with provided handler and exemplar
func CreateQueue(name string, handle HandlerFunc, exemplar interface{}) Queue {
	q := setting.GetQueueSettings(name)
	opts := make(map[string]interface{})
	opts["Name"] = name
	opts["QueueLength"] = q.Length
	opts["BatchLength"] = q.BatchLength
	opts["DataDir"] = q.DataDir
	opts["Addresses"] = q.Addresses
	opts["Network"] = q.Network
	opts["Password"] = q.Password
	opts["DBIndex"] = q.DBIndex
	opts["QueueName"] = q.QueueName
	opts["Workers"] = q.Workers
	opts["MaxWorkers"] = q.MaxWorkers
	opts["BlockTimeout"] = q.BlockTimeout
	opts["BoostTimeout"] = q.BoostTimeout
	opts["BoostWorkers"] = q.BoostWorkers

	typ, err := validType(q.Type)
	if err != nil {
		log.Error("Invalid type %s provided for queue named %s defaulting to %s", q.Type, name, string(typ))
	}

	cfg, err := json.Marshal(opts)
	if err != nil {
		log.Error("Unable to marshall generic options: %v Error: %v", opts, err)
		log.Error("Unable to create queue for %s", name, err)
		return nil
	}

	returnable, err := NewQueue(typ, handle, cfg, exemplar)
	if q.WrapIfNecessary && err != nil {
		log.Warn("Unable to create queue for %s: %v", name, err)
		log.Warn("Attempting to create wrapped queue")
		returnable, err = NewQueue(WrappedQueueType, handle, WrappedQueueConfiguration{
			Underlying:  Type(q.Type),
			Timeout:     q.Timeout,
			MaxAttempts: q.MaxAttempts,
			Config:      cfg,
			QueueLength: q.Length,
		}, exemplar)
	}
	if err != nil {
		log.Error("Unable to create queue for %s: %v", name, err)
		return nil
	}
	return returnable
}
