package uuid

/****************
 * Date: 21/06/15
 * Time: 5:48 PM
 ***************/

import (
	"encoding/gob"
	"log"
	"os"
	"time"
)

func init() {
	gob.Register(stateEntity{})
}

func SetupFileSystemStateSaver(pConfig StateSaverConfig) {
	saver := &FileSystemSaver{}
	saver.saveReport = pConfig.SaveReport
	saver.saveSchedule = int64(pConfig.SaveSchedule)
	SetupCustomStateSaver(saver)
}

// A wrapper for default setup of the FileSystemStateSaver
type StateSaverConfig struct {

	// Print save log
	SaveReport bool

	// Save every x nanoseconds
	SaveSchedule time.Duration
}

// ***********************************************  StateEntity

// StateEntity acts as a marshaller struct for the state
type stateEntity struct {
	Past     Timestamp
	Node     []byte
	Sequence uint16
}

// This implements the StateSaver interface for UUIDs
type FileSystemSaver struct {
	cache        *os.File
	saveState    uint64
	saveReport   bool
	saveSchedule int64
}

// Saves the current state of the generator
// If the scheduled file save is reached then the file is synced
func (o *FileSystemSaver) Save(pState *State) {
	if pState.past >= pState.next {
		err := o.open()
		defer o.cache.Close()
		if err != nil {
			log.Println("uuid.State.save:", err)
			return
		}
		// do the save
		o.encode(pState)
		// a tick is 100 nano seconds
		pState.next = pState.past + Timestamp(o.saveSchedule / 100)
		if o.saveReport {
			log.Printf("UUID STATE: SAVED %d", pState.past)
		}
	}
}

func (o *FileSystemSaver) Init(pState *State) {
	pState.saver = o
	err := o.open()
	defer o.cache.Close()
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("'%s' created\n", "uuid.SaveState")
			var err error
			o.cache, err = os.Create(os.TempDir() + "/state.unique")
			if err != nil {
				log.Println("uuid.State.init: SaveState error:", err)
				goto pastInit
			}
			o.encode(pState)
		} else {
			log.Println("uuid.State.init: SaveState error:", err)
			goto pastInit
		}
	}
	err = o.decode(pState)
	if err != nil {
		goto pastInit
	}
	pState.randomSequence = false
pastInit:
	if timestamp() <= pState.past {
		pState.sequence++
	}
	pState.next = pState.past
}

func (o *FileSystemSaver) reset() {
	o.cache.Seek(0, 0)
}

func (o *FileSystemSaver) open() error {
	var err error
	o.cache, err = os.OpenFile(os.TempDir()+"/state.unique", os.O_RDWR, os.ModeExclusive)
	return err
}

// Encodes State generator data into a saved file
func (o *FileSystemSaver) encode(pState *State) {
	// ensure reader state is ready for use
	o.reset()
	enc := gob.NewEncoder(o.cache)
	// Wrap private State data into the StateEntity
	err := enc.Encode(&stateEntity{pState.past, pState.node, pState.sequence})
	if err != nil {
		log.Panic("UUID.encode error:", err)
	}
}

// Decodes StateEntity data into the main State
func (o *FileSystemSaver) decode(pState *State) error {
	o.reset()
	dec := gob.NewDecoder(o.cache)
	entity := stateEntity{}
	err := dec.Decode(&entity)
	if err != nil {
		log.Println("uuid.decode error:", err)
		return err
	}
	pState.past = entity.Past
	pState.node = entity.Node
	pState.sequence = entity.Sequence
	return nil
}
