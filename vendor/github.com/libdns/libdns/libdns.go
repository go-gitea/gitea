// Package libdns defines the core interfaces that should be implemented
// by DNS provider clients. They are small and idiomatic Go interfaces with
// well-defined semantics.
//
// All interface implementations must be safe for concurrent/parallel use.
// For example, if AppendRecords() is called at the same time and two API
// requests are made to the provider at the same time, the result of both
// requests must be visible after they both complete; if the provider does
// not synchronize the writing of the zone file and one request overwrites
// the other, then the client implementation must take care to synchronize
// on behalf of the incompetent provider. This synchronization need not be
// global, for example: the scope of synchronization might only need to be
// within the same zone, allowing multiple requests at once as long as all
// of them are for different zones. (Exact logic depends on the provider.)
package libdns

import (
	"context"
	"time"
)

// RecordGetter can get records from a DNS zone.
type RecordGetter interface {
	// GetRecords returns all the records in the DNS zone.
	//
	// Implementations must honor context cancellation and be safe for
	// concurrent use.
	GetRecords(ctx context.Context, zone string) ([]Record, error)
}

// RecordAppender can non-destructively add new records to a DNS zone.
type RecordAppender interface {
	// AppendRecords creates the requested records in the given zone
	// and returns the populated records that were created. It never
	// changes existing records.
	//
	// Implementations must honor context cancellation and be safe for
	// concurrent use.
	AppendRecords(ctx context.Context, zone string, recs []Record) ([]Record, error)
}

// RecordSetter can set new or update existing records in a DNS zone.
type RecordSetter interface {
	// SetRecords updates the zone so that the records described in the
	// input are reflected in the output. It may create or overwrite
	// records or -- depending on the record type -- delete records to
	// maintain parity with the input. No other records are affected.
	// It returns the records which were set.
	//
	// Records that have an ID associating it with a particular resource
	// on the provider will be directly replaced. If no ID is given, this
	// method may use what information is given to do lookups and will
	// ensure that only necessary changes are made to the zone.
	//
	// Implementations must honor context cancellation and be safe for
	// concurrent use.
	SetRecords(ctx context.Context, zone string, recs []Record) ([]Record, error)
}

// RecordDeleter can delete records from a DNS zone.
type RecordDeleter interface {
	// DeleteRecords deletes the given records from the zone if they exist.
	// It returns the records that were deleted.
	//
	// Records that have an ID to associate it with a particular resource on
	// the provider will be directly deleted. If no ID is given, this method
	// may use what information is given to do lookups and delete only
	// matching records.
	//
	// Implementations must honor context cancellation and be safe for
	// concurrent use.
	DeleteRecords(ctx context.Context, zone string, recs []Record) ([]Record, error)
}

// Record is a generalized representation of a DNS record.
type Record struct {
	// provider-specific metadata
	ID string

	// general record fields
	Type  string
	Name  string
	Value string
	TTL   time.Duration
}
