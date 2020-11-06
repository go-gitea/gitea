package pam

import "sync"

var cb struct {
	sync.Mutex
	m map[int]interface{}
	c int
}

func init() {
	cb.m = make(map[int]interface{})
}

func cbAdd(v interface{}) int {
	cb.Lock()
	defer cb.Unlock()
	cb.c++
	cb.m[cb.c] = v
	return cb.c
}

func cbGet(c int) interface{} {
	cb.Lock()
	defer cb.Unlock()
	if v, ok := cb.m[c]; ok {
		return v
	}
	panic("Callback pointer not found")
}

func cbDelete(c int) {
	cb.Lock()
	defer cb.Unlock()
	if _, ok := cb.m[c]; !ok {
		panic("Callback pointer not found")
	}
	delete(cb.m, c)
}
