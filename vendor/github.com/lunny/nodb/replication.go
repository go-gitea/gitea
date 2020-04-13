package nodb

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"time"

	"github.com/lunny/log"
	"github.com/lunny/nodb/store/driver"
)

const (
	maxReplBatchNum = 100
	maxReplLogSize  = 1 * 1024 * 1024
)

var (
	ErrSkipEvent = errors.New("skip to next event")
)

var (
	errInvalidBinLogEvent = errors.New("invalid binglog event")
	errInvalidBinLogFile  = errors.New("invalid binlog file")
)

type replBatch struct {
	wb     driver.IWriteBatch
	events [][]byte
	l      *Nodb

	lastHead *BinLogHead
}

func (b *replBatch) Commit() error {
	b.l.commitLock.Lock()
	defer b.l.commitLock.Unlock()

	err := b.wb.Commit()
	if err != nil {
		b.Rollback()
		return err
	}

	if b.l.binlog != nil {
		if err = b.l.binlog.Log(b.events...); err != nil {
			b.Rollback()
			return err
		}
	}

	b.events = [][]byte{}
	b.lastHead = nil

	return nil
}

func (b *replBatch) Rollback() error {
	b.wb.Rollback()
	b.events = [][]byte{}
	b.lastHead = nil
	return nil
}

func (l *Nodb) replicateEvent(b *replBatch, event []byte) error {
	if len(event) == 0 {
		return errInvalidBinLogEvent
	}

	b.events = append(b.events, event)

	logType := uint8(event[0])
	switch logType {
	case BinLogTypePut:
		return l.replicatePutEvent(b, event)
	case BinLogTypeDeletion:
		return l.replicateDeleteEvent(b, event)
	default:
		return errInvalidBinLogEvent
	}
}

func (l *Nodb) replicatePutEvent(b *replBatch, event []byte) error {
	key, value, err := decodeBinLogPut(event)
	if err != nil {
		return err
	}

	b.wb.Put(key, value)

	return nil
}

func (l *Nodb) replicateDeleteEvent(b *replBatch, event []byte) error {
	key, err := decodeBinLogDelete(event)
	if err != nil {
		return err
	}

	b.wb.Delete(key)

	return nil
}

func ReadEventFromReader(rb io.Reader, f func(head *BinLogHead, event []byte) error) error {
	head := &BinLogHead{}
	var err error

	for {
		if err = head.Read(rb); err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}

		var dataBuf bytes.Buffer

		if _, err = io.CopyN(&dataBuf, rb, int64(head.PayloadLen)); err != nil {
			return err
		}

		err = f(head, dataBuf.Bytes())
		if err != nil && err != ErrSkipEvent {
			return err
		}
	}

	return nil
}

func (l *Nodb) ReplicateFromReader(rb io.Reader) error {
	b := new(replBatch)

	b.wb = l.ldb.NewWriteBatch()
	b.l = l

	f := func(head *BinLogHead, event []byte) error {
		if b.lastHead == nil {
			b.lastHead = head
		} else if !b.lastHead.InSameBatch(head) {
			if err := b.Commit(); err != nil {
				log.Fatal("replication error %s, skip to next", err.Error())
				return ErrSkipEvent
			}
			b.lastHead = head
		}

		err := l.replicateEvent(b, event)
		if err != nil {
			log.Fatal("replication error %s, skip to next", err.Error())
			return ErrSkipEvent
		}
		return nil
	}

	err := ReadEventFromReader(rb, f)
	if err != nil {
		b.Rollback()
		return err
	}
	return b.Commit()
}

func (l *Nodb) ReplicateFromData(data []byte) error {
	rb := bytes.NewReader(data)

	err := l.ReplicateFromReader(rb)

	return err
}

func (l *Nodb) ReplicateFromBinLog(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}

	rb := bufio.NewReaderSize(f, 4096)

	err = l.ReplicateFromReader(rb)

	f.Close()

	return err
}

// try to read events, if no events read, try to wait the new event singal until timeout seconds
func (l *Nodb) ReadEventsToTimeout(info *BinLogAnchor, w io.Writer, timeout int) (n int, err error) {
	lastIndex := info.LogFileIndex
	lastPos := info.LogPos

	n = 0
	if l.binlog == nil {
		//binlog not supported
		info.LogFileIndex = 0
		info.LogPos = 0
		return
	}

	n, err = l.ReadEventsTo(info, w)
	if err == nil && info.LogFileIndex == lastIndex && info.LogPos == lastPos {
		//no events read
		select {
		case <-l.binlog.Wait():
		case <-time.After(time.Duration(timeout) * time.Second):
		}
		return l.ReadEventsTo(info, w)
	}
	return
}

func (l *Nodb) ReadEventsTo(info *BinLogAnchor, w io.Writer) (n int, err error) {
	n = 0
	if l.binlog == nil {
		//binlog not supported
		info.LogFileIndex = 0
		info.LogPos = 0
		return
	}

	index := info.LogFileIndex
	offset := info.LogPos

	filePath := l.binlog.FormatLogFilePath(index)

	var f *os.File
	f, err = os.Open(filePath)
	if os.IsNotExist(err) {
		lastIndex := l.binlog.LogFileIndex()

		if index == lastIndex {
			//no binlog at all
			info.LogPos = 0
		} else {
			//slave binlog info had lost
			info.LogFileIndex = -1
		}
	}

	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}

	defer f.Close()

	var fileSize int64
	st, _ := f.Stat()
	fileSize = st.Size()

	if fileSize == info.LogPos {
		return
	}

	if _, err = f.Seek(offset, os.SEEK_SET); err != nil {
		//may be invliad seek offset
		return
	}

	var lastHead *BinLogHead = nil

	head := &BinLogHead{}

	batchNum := 0

	for {
		if err = head.Read(f); err != nil {
			if err == io.EOF {
				//we will try to use next binlog
				if index < l.binlog.LogFileIndex() {
					info.LogFileIndex += 1
					info.LogPos = 0
				}
				err = nil
				return
			} else {
				return
			}

		}

		if lastHead == nil {
			lastHead = head
			batchNum++
		} else if !lastHead.InSameBatch(head) {
			lastHead = head
			batchNum++
			if batchNum > maxReplBatchNum || n > maxReplLogSize {
				return
			}
		}

		if err = head.Write(w); err != nil {
			return
		}

		if _, err = io.CopyN(w, f, int64(head.PayloadLen)); err != nil {
			return
		}

		n += (head.Len() + int(head.PayloadLen))
		info.LogPos = info.LogPos + int64(head.Len()) + int64(head.PayloadLen)
	}

	return
}
