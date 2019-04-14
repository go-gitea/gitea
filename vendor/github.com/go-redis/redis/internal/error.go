package internal

import (
	"io"
	"net"
	"strings"

	"github.com/go-redis/redis/internal/proto"
)

func IsRetryableError(err error, retryTimeout bool) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return retryTimeout
		}
		return true
	}
	s := err.Error()
	if s == "ERR max number of clients reached" {
		return true
	}
	if strings.HasPrefix(s, "LOADING ") {
		return true
	}
	if strings.HasPrefix(s, "READONLY ") {
		return true
	}
	if strings.HasPrefix(s, "CLUSTERDOWN ") {
		return true
	}
	return false
}

func IsRedisError(err error) bool {
	_, ok := err.(proto.RedisError)
	return ok
}

func IsBadConn(err error, allowTimeout bool) bool {
	if err == nil {
		return false
	}
	if IsRedisError(err) {
		// #790
		return IsReadOnlyError(err)
	}
	if allowTimeout {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return false
		}
	}
	return true
}

func IsMovedError(err error) (moved bool, ask bool, addr string) {
	if !IsRedisError(err) {
		return
	}

	s := err.Error()
	if strings.HasPrefix(s, "MOVED ") {
		moved = true
	} else if strings.HasPrefix(s, "ASK ") {
		ask = true
	} else {
		return
	}

	ind := strings.LastIndex(s, " ")
	if ind == -1 {
		return false, false, ""
	}
	addr = s[ind+1:]
	return
}

func IsLoadingError(err error) bool {
	return strings.HasPrefix(err.Error(), "LOADING ")
}

func IsReadOnlyError(err error) bool {
	return strings.HasPrefix(err.Error(), "READONLY ")
}
