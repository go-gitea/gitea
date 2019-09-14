// +build windows plan9 solaris appengine

package flags

func getTerminalColumns() int {
	return 80
}
