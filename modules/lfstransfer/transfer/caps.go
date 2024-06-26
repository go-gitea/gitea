package transfer

// Version is the git-lfs-transfer protocol version number.
const Version = "1"

// Capabilities is a list of Git LFS capabilities supported by this package.
var Capabilities = []string{
	"version=" + Version,
	"locking",
}
