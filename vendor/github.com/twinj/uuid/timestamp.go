package uuid

/****************
 * Date: 14/02/14
 * Time: 7:46 PM
 ***************/

import (
	"time"
)

const (
	// A tick is 100 ns
	ticksPerSecond = 10000000

	// Difference between
	gregorianToUNIXOffset uint64 = 0x01B21DD213814000

	// set the following to the number of 100ns ticks of the actual
	// resolution of your system's clock
	idsPerTimestamp = 1024
)

var (
	lastTimestamp    Timestamp
	idsThisTimestamp = idsPerTimestamp
)

// **********************************************  Timestamp

type Timestamp uint64

// TODO Create c version same as package runtime and time
func Now() (sec int64, nsec int32) {
	t := time.Now()
	sec = t.Unix()
	nsec = int32(t.Nanosecond())
	return
}

// Converts Unix formatted time to RFC4122 UUID formatted times
// UUID UTC base time is October 15, 1582.
// Unix base time is January 1, 1970.
// Converts time to 100 nanosecond ticks since epoch
// There are 1000000000 nanoseconds in a second,
// 1000000000 / 100 = 10000000 tiks per second
func timestamp() Timestamp {
	sec, nsec := Now()
	return Timestamp(uint64(sec)*ticksPerSecond +
		uint64(nsec)/100 + gregorianToUNIXOffset)
}

func (o Timestamp) Unix() time.Time {
	t := uint64(o) - gregorianToUNIXOffset
	return time.Unix(0, int64(t*100))
}

// Get time as 60-bit 100ns ticks since UUID epoch.
// Compensate for the fact that real clock resolution is
// less than 100ns.
func currentUUIDTimestamp() Timestamp {
	var timeNow Timestamp
	for {
		timeNow = timestamp()

		// if clock reading changed since last UUID generated
		if lastTimestamp != timeNow {
			// reset count of UUIDs with this timestamp
			idsThisTimestamp = 0
			lastTimestamp = timeNow
			break
		}
		if idsThisTimestamp < idsPerTimestamp {
			idsThisTimestamp++
			break
		}
		// going too fast for the clock; spin
	}
	// add the count of UUIDs to low order bits of the clock reading
	return timeNow + Timestamp(idsThisTimestamp)
}
