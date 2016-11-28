package uuid

/****************
 * Date: 14/02/14
 * Time: 7:43 PM
 ***************/

import (
	"bytes"
	"log"
	seed "math/rand"
	"net"
	"sync"
)


// **************************************************** State

func SetupCustomStateSaver(pSaver StateSaver) {
	state.Lock()
	pSaver.Init(&state)
	state.init()
	state.Unlock()
}

// Holds package information about the current
// state of the UUID generator
type State struct {

	// A flag which informs whether to
	// randomly create a node id
	randomNode bool

	// A flag which informs whether to
	// randomly create the sequence
	randomSequence bool

	// the last time UUID was saved
	past Timestamp

	// the next time the state will be saved
	next Timestamp

	// the last node which saved a UUID
	node []byte

	// An iterated value to help ensure different
	// values across the same domain
	sequence uint16

	sync.Mutex

	// save state interface
	saver StateSaver
}

// Changes the state with current data
// Compares the current found node to the last node stored,
// If they are the same or randomSequence is already set due
// to an earlier read issue then the sequence is randomly generated
// else if there is an issue with the time the sequence is incremented
func (o *State) read(pNow Timestamp, pNode net.HardwareAddr) {
	if bytes.Equal([]byte(pNode), o.node) || o.randomSequence {
		o.sequence = uint16(seed.Int()) & 0x3FFF
	} else if pNow < o.past {
		o.sequence++
	}
	o.past = pNow
	o.node = pNode
}

func (o *State) persist() {
	if o.saver != nil {
		o.saver.Save(o)
	}
}

// Initialises the UUID state when the package is first loaded
// it first attempts to decode the file state into State
// if this file does not exist it will create the file and do a flush
// of the random state which gets loaded at package runtime
// second it will attempt to resolve the current hardware address nodeId
// thirdly it will check the state of the clock
func (o *State) init() {
	if o.saver != nil {
		intfcs, err := net.Interfaces()
		if err != nil {
			log.Println("uuid.State.init: address error: will generate random node id instead", err)
			return
		}
		a := getHardwareAddress(intfcs)
		if a == nil {
			log.Println("uuid.State.init: address error: will generate random node id instead", err)
			return
		}
		// Don't use random as we have a real address
		o.randomSequence = false
		if bytes.Equal([]byte(a), state.node) {
			state.sequence++
		}
		state.node = a
		state.randomNode = false
	}
}

func getHardwareAddress(pInterfaces []net.Interface) net.HardwareAddr {
	for _, inter := range pInterfaces {
		// Initially I could multicast out the Flags to get
		// whether the interface was up but started failing
		if (inter.Flags & (1 << net.FlagUp)) != 0 {
			//if inter.Flags.String() != "0" {
			if addrs, err := inter.Addrs(); err == nil {
				for _, addr := range addrs {
					if addr.String() != "0.0.0.0" && !bytes.Equal([]byte(inter.HardwareAddr), make([]byte, len(inter.HardwareAddr))) {
						return inter.HardwareAddr
					}
				}
			}
		}
	}
	return nil
}

// *********************************************** StateSaver interface

// Use this interface to setup a custom state saver if you wish to have
// v1 UUIDs based on your node id and constant time.
type StateSaver interface {
	// Init is run if Setup() is false
	// Init should setup the system to save the state
	Init(*State)

	// Save saves the state and is called only if const V1Save and
	// Setup() is true
	Save(*State)
}

