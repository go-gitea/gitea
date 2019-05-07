package driver

type BatchPuter interface {
	BatchPut([]Write) error
}

type Write struct {
	Key   []byte
	Value []byte
}

type WriteBatch struct {
	batch BatchPuter
	wb    []Write
}

func (w *WriteBatch) Put(key, value []byte) {
	if value == nil {
		value = []byte{}
	}
	w.wb = append(w.wb, Write{key, value})
}

func (w *WriteBatch) Delete(key []byte) {
	w.wb = append(w.wb, Write{key, nil})
}

func (w *WriteBatch) Commit() error {
	return w.batch.BatchPut(w.wb)
}

func (w *WriteBatch) Rollback() error {
	w.wb = w.wb[0:0]
	return nil
}

func NewWriteBatch(puter BatchPuter) IWriteBatch {
	return &WriteBatch{puter, []Write{}}
}
