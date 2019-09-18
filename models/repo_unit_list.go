package models

import "sync"

// RepoUnitList is a thread-safe container for a list of RepoUnit
type RepoUnitList struct {
	sync.RWMutex
	list []*RepoUnit
}

// NewRepoUnitList creates a RepoUnitList from a slice of *UnitRepo.
func NewRepoUnitList(us []*RepoUnit) *RepoUnitList {
	return &RepoUnitList{
		list: us,
	}
}

// Load reads i-th element from the list
func (l *RepoUnitList) Load(i int) *RepoUnit {
	l.RLock()
	defer l.RUnlock()
	return l.list[i]
}

// Append appends a element to the list
func (l *RepoUnitList) Append(u *RepoUnit) {
	l.Lock()
	defer l.Unlock()
	l.list = append(l.list, u)
}

// Len returns the length of the list
func (l *RepoUnitList) Len() int {
	l.RLock()
	defer l.RUnlock()
	return len(l.list)
}

// Range iterates through the elements of the list like sync.Map.Range.
func (l *RepoUnitList) Range(f func(i int, u *RepoUnit) bool) {

	l.RLock()
	for i, v := range l.list {
		// Despite the cost of calling lock/unlock in a loop,
		// we have to release the lock during the execution of the callback.
		// Otherwise, if f tries to acquire the lock, a deadlock will happen.
		l.RUnlock()
		ok := f(i, v)
		l.RLock()

		if !ok {
			break
		}
	}
	l.RUnlock()
}
