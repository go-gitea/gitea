package websocket

import (
	"fmt"

	"github.com/olahol/melody"
)

type sessionData struct {
	uid        int64
	unregister func()
}

func getSessionData(s *melody.Session) (*sessionData, error) {
	_data, ok := s.Get("data")
	if !ok {
		return nil, fmt.Errorf("no session data")
	}

	data, ok := _data.(*sessionData)
	if !ok {
		return nil, fmt.Errorf("invalid session data")
	}

	return data, nil
}
