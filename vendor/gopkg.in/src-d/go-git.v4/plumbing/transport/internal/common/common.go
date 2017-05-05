// Package common implements the git pack protocol with a pluggable transport.
// This is a low-level package to implement new transports. Use a concrete
// implementation instead (e.g. http, file, ssh).
//
// A simple example of usage can be found in the file package.
package common

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

const (
	readErrorSecondsTimeout = 10
	errLinesBuffer          = 1000
)

var (
	ErrTimeoutExceeded = errors.New("timeout exceeded")
)

// Commander creates Command instances. This is the main entry point for
// transport implementations.
type Commander interface {
	// Command creates a new Command for the given git command and
	// endpoint. cmd can be git-upload-pack or git-receive-pack. An
	// error should be returned if the endpoint is not supported or the
	// command cannot be created (e.g. binary does not exist, connection
	// cannot be established).
	Command(cmd string, ep transport.Endpoint) (Command, error)
}

// Command is used for a single command execution.
// This interface is modeled after exec.Cmd and ssh.Session in the standard
// library.
type Command interface {
	// SetAuth sets the authentication method.
	SetAuth(transport.AuthMethod) error
	// StderrPipe returns a pipe that will be connected to the command's
	// standard error when the command starts. It should not be called after
	// Start.
	StderrPipe() (io.Reader, error)
	// StdinPipe returns a pipe that will be connected to the command's
	// standard input when the command starts. It should not be called after
	// Start. The pipe should be closed when no more input is expected.
	StdinPipe() (io.WriteCloser, error)
	// StdoutPipe returns a pipe that will be connected to the command's
	// standard output when the command starts. It should not be called after
	// Start.
	StdoutPipe() (io.Reader, error)
	// Start starts the specified command. It does not wait for it to
	// complete.
	Start() error
	// Wait waits for the command to exit. It must have been started by
	// Start. The returned error is nil if the command runs, has no
	// problems copying stdin, stdout, and stderr, and exits with a zero
	// exit status.
	Wait() error
	// Close closes the command and releases any resources used by it. It
	// can be called to forcibly finish the command without calling to Wait
	// or to release resources after calling Wait.
	Close() error
}

type client struct {
	cmdr Commander
}

// NewClient creates a new client using the given Commander.
func NewClient(runner Commander) transport.Client {
	return &client{runner}
}

// NewFetchPackSession creates a new FetchPackSession.
func (c *client) NewFetchPackSession(ep transport.Endpoint) (
	transport.FetchPackSession, error) {

	return c.newSession(transport.UploadPackServiceName, ep)
}

// NewSendPackSession creates a new SendPackSession.
func (c *client) NewSendPackSession(ep transport.Endpoint) (
	transport.SendPackSession, error) {

	return c.newSession(transport.ReceivePackServiceName, ep)
}

type session struct {
	Stdin   io.WriteCloser
	Stdout  io.Reader
	Command Command

	isReceivePack bool
	advRefs       *packp.AdvRefs
	packRun       bool
	finished      bool
	errLines      chan string
}

func (c *client) newSession(s string, ep transport.Endpoint) (*session, error) {
	cmd, err := c.cmdr.Command(s, ep)
	if err != nil {
		return nil, err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &session{
		Stdin:         stdin,
		Stdout:        stdout,
		Command:       cmd,
		errLines:      c.listenErrors(stderr),
		isReceivePack: s == transport.ReceivePackServiceName,
	}, nil
}

func (c *client) listenErrors(r io.Reader) chan string {
	if r == nil {
		return nil
	}

	errLines := make(chan string, errLinesBuffer)
	go func() {
		s := bufio.NewScanner(r)
		for s.Scan() {
			line := string(s.Bytes())
			errLines <- line
		}
	}()

	return errLines
}

// SetAuth delegates to the command's SetAuth.
func (s *session) SetAuth(auth transport.AuthMethod) error {
	return s.Command.SetAuth(auth)
}

// AdvertisedReferences retrieves the advertised references from the server.
func (s *session) AdvertisedReferences() (*packp.AdvRefs, error) {
	if s.advRefs != nil {
		return s.advRefs, nil
	}

	ar := packp.NewAdvRefs()
	if err := ar.Decode(s.Stdout); err != nil {
		if err := s.handleAdvRefDecodeError(err); err != nil {
			return nil, err
		}
	}

	transport.FilterUnsupportedCapabilities(ar.Capabilities)
	s.advRefs = ar
	return ar, nil
}

func (s *session) handleAdvRefDecodeError(err error) error {
	// If repository is not found, we get empty stdout and server writes an
	// error to stderr.
	if err == packp.ErrEmptyInput {
		if err := s.checkNotFoundError(); err != nil {
			return err
		}

		return io.ErrUnexpectedEOF
	}

	// For empty (but existing) repositories, we get empty advertised-references
	// message. But valid. That is, it includes at least a flush.
	if err == packp.ErrEmptyAdvRefs {
		// Empty repositories are valid for git-receive-pack.
		if s.isReceivePack {
			return nil
		}

		if err := s.finish(); err != nil {
			return err
		}

		return transport.ErrEmptyRemoteRepository
	}

	// Some server sends the errors as normal content (git protocol), so when
	// we try to decode it fails, we need to check the content of it, to detect
	// not found errors
	if uerr, ok := err.(*packp.ErrUnexpectedData); ok {
		if isRepoNotFoundError(string(uerr.Data)) {
			return transport.ErrRepositoryNotFound
		}
	}

	return err
}

// FetchPack performs a request to the server to fetch a packfile. A reader is
// returned with the packfile content. The reader must be closed after reading.
func (s *session) FetchPack(req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	if req.IsEmpty() {
		return nil, transport.ErrEmptyUploadPackRequest
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	if _, err := s.AdvertisedReferences(); err != nil {
		return nil, err
	}

	s.packRun = true

	if err := fetchPack(s.Stdin, s.Stdout, req); err != nil {
		return nil, err
	}

	r, err := ioutil.NonEmptyReader(s.Stdout)
	if err == ioutil.ErrEmptyReader {
		if c, ok := s.Stdout.(io.Closer); ok {
			_ = c.Close()
		}

		return nil, transport.ErrEmptyUploadPackRequest
	}

	if err != nil {
		return nil, err
	}

	wc := &waitCloser{s.Command}
	rc := ioutil.NewReadCloser(r, wc)

	return DecodeUploadPackResponse(rc, req)
}

func (s *session) SendPack(req *packp.ReferenceUpdateRequest) (*packp.ReportStatus, error) {
	if _, err := s.AdvertisedReferences(); err != nil {
		return nil, err
	}

	s.packRun = true

	if err := req.Encode(s.Stdin); err != nil {
		return nil, err
	}

	if !req.Capabilities.Supports(capability.ReportStatus) {
		// If we have neither report-status or sideband, we can only
		// check return value error.
		return nil, s.Command.Wait()
	}

	report := packp.NewReportStatus()
	if err := report.Decode(s.Stdout); err != nil {
		return nil, err
	}

	if !report.Ok() {
		return report, fmt.Errorf("report status: %s", report.UnpackStatus)
	}

	return report, s.Command.Wait()
}

func (s *session) finish() error {
	if s.finished {
		return nil
	}

	s.finished = true

	// If we did not run fetch-pack or send-pack, we close the connection
	// gracefully by sending a flush packet to the server. If the server
	// operates correctly, it will exit with status 0.
	if !s.packRun {
		_, err := s.Stdin.Write(pktline.FlushPkt)
		return err
	}

	return nil
}

func (s *session) Close() error {
	if err := s.finish(); err != nil {
		_ = s.Command.Close()
		return nil
	}

	return s.Command.Close()
}

func (s *session) checkNotFoundError() error {
	t := time.NewTicker(time.Second * readErrorSecondsTimeout)
	defer t.Stop()

	select {
	case <-t.C:
		return ErrTimeoutExceeded
	case line, ok := <-s.errLines:
		if !ok {
			return nil
		}

		if isRepoNotFoundError(line) {
			return transport.ErrRepositoryNotFound
		}

		return fmt.Errorf("unknown error: %s", line)
	}
}

var (
	githubRepoNotFoundErr    = "ERROR: Repository not found."
	bitbucketRepoNotFoundErr = "conq: repository does not exist."
	localRepoNotFoundErr     = "does not appear to be a git repository"
	gitProtocolNotFoundErr   = "ERR \n  Repository not found."
)

func isRepoNotFoundError(s string) bool {
	if strings.HasPrefix(s, githubRepoNotFoundErr) {
		return true
	}

	if strings.HasPrefix(s, bitbucketRepoNotFoundErr) {
		return true
	}

	if strings.HasSuffix(s, localRepoNotFoundErr) {
		return true
	}

	if strings.HasPrefix(s, gitProtocolNotFoundErr) {
		return true
	}

	return false
}

var (
	nak = []byte("NAK")
	eol = []byte("\n")
)

// fetchPack implements the git-fetch-pack protocol.
//
// TODO support multi_ack mode
// TODO support multi_ack_detailed mode
// TODO support acks for common objects
// TODO build a proper state machine for all these processing options
func fetchPack(w io.WriteCloser, r io.Reader, req *packp.UploadPackRequest) error {
	if err := req.UploadRequest.Encode(w); err != nil {
		return fmt.Errorf("sending upload-req message: %s", err)
	}

	if err := req.UploadHaves.Encode(w); err != nil {
		return fmt.Errorf("sending haves message: %s", err)
	}

	if err := sendDone(w); err != nil {
		return fmt.Errorf("sending done message: %s", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("closing input: %s", err)
	}

	return nil
}

func sendDone(w io.Writer) error {
	e := pktline.NewEncoder(w)

	return e.Encodef("done\n")
}

// DecodeUploadPackResponse decodes r into a new packp.UploadPackResponse
func DecodeUploadPackResponse(r io.ReadCloser, req *packp.UploadPackRequest) (
	*packp.UploadPackResponse, error,
) {
	res := packp.NewUploadPackResponse(req)
	if err := res.Decode(r); err != nil {
		return nil, fmt.Errorf("error decoding upload-pack response: %s", err)
	}

	return res, nil
}

type waitCloser struct {
	Command Command
}

// Close waits until the command exits and returns error, if any.
func (c *waitCloser) Close() error {
	return c.Command.Wait()
}
