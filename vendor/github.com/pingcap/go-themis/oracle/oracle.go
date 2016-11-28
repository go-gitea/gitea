package oracle

type Oracle interface {
	GetTimestamp() (uint64, error)
	IsExpired(lockTimestamp uint64, TTL uint64) bool
}
