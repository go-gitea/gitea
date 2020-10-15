// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"encoding/json"
	"sort"
	"sync"
)

var manager *Manager

// Manager is the storage manager
type Manager struct {
	mutex sync.Mutex

	storage map[string]ManagedStorage
}

// ManagedStorage represents an ObjectStorage coupled with its name, type and configuration
type ManagedStorage struct {
	Name          string
	Type          string
	Config        interface{}
	ObjectStorage ObjectStorage
}

// ConfigAsString returns the config as a string
func (ms *ManagedStorage) ConfigAsString() string {
	bs, err := json.Marshal(ms.Config)
	if err != nil {
		return ""
	}
	return string(bs)
}

func init() {
	_ = GetManager()
}

// GetManager returns a Manager and initializes one as singleton is there's none yet
func GetManager() *Manager {
	if manager == nil {
		manager = &Manager{
			storage: make(map[string]ManagedStorage),
		}
	}
	return manager
}

// Get will return the storage for the provided name
func (m *Manager) Get(name string) ObjectStorage {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.storage[name].ObjectStorage
}

// Add will add the storage to the manager
func (m *Manager) Add(name, typ string, config interface{}, storage ObjectStorage) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.storage[name] = ManagedStorage{
		Name:          name,
		Type:          typ,
		Config:        config,
		ObjectStorage: storage,
	}
}

// GetAll will return a duplicate storage map
func (m *Manager) GetAll() []ManagedStorage {
	m.mutex.Lock()
	returnable := make([]ManagedStorage, 0, len(m.storage))
	for _, storage := range m.storage {
		returnable = append(returnable, storage)
	}
	m.mutex.Unlock()
	sort.Slice(returnable, func(i, j int) bool {
		return returnable[i].Name < returnable[j].Name
	})
	return returnable
}
