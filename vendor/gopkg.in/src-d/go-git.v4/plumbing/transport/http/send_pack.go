package http

import (
	"errors"
	"net/http"

	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

var errSendPackNotSupported = errors.New("send-pack not supported yet")

type sendPackSession struct {
	*session
}

func newSendPackSession(c *http.Client, ep transport.Endpoint) transport.SendPackSession {
	return &sendPackSession{&session{}}
}

func (s *sendPackSession) AdvertisedReferences() (*packp.AdvRefs, error) {

	return nil, errSendPackNotSupported
}

func (s *sendPackSession) SendPack(*packp.ReferenceUpdateRequest) (
	*packp.ReportStatus, error) {

	return nil, errSendPackNotSupported
}
