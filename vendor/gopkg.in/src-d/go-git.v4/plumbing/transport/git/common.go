package git

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/internal/common"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

var (
	errAlreadyConnected = errors.New("tcp connection already connected")
)

// DefaultClient is the default git client.
var DefaultClient = common.NewClient(&runner{})

type runner struct{}

// Command returns a new Command for the given cmd in the given Endpoint
func (r *runner) Command(cmd string, ep transport.Endpoint) (common.Command, error) {
	c := &command{command: cmd, endpoint: ep}
	if err := c.connect(); err != nil {
		return nil, err
	}

	return c, nil
}

type command struct {
	conn      net.Conn
	connected bool
	command   string
	endpoint  transport.Endpoint
}

// SetAuth cannot be called since git protocol doesn't support authentication
func (c *command) SetAuth(auth transport.AuthMethod) error {
	return transport.ErrInvalidAuthMethod
}

// Start executes the command sending the required message to the TCP connection
func (c *command) Start() error {
	cmd := endpointToCommand(c.command, c.endpoint)

	e := pktline.NewEncoder(c.conn)
	return e.Encode([]byte(cmd))
}

func (c *command) connect() error {
	if c.connected {
		return errAlreadyConnected
	}

	var err error
	c.conn, err = net.Dial("tcp", c.getHostWithPort())
	if err != nil {
		return err
	}

	c.connected = true
	return nil
}

func (c *command) getHostWithPort() string {
	host := c.endpoint.Host
	if strings.Index(c.endpoint.Host, ":") == -1 {
		host += ":9418"
	}

	return host
}

// StderrPipe git protocol doesn't have any dedicated error channel
func (c *command) StderrPipe() (io.Reader, error) {
	return nil, nil
}

// StdinPipe return the underlying connection as WriteCloser, wrapped to prevent
// call to the Close function from the connection, a command execution in git
// protocol can't be closed or killed
func (c *command) StdinPipe() (io.WriteCloser, error) {
	return ioutil.WriteNopCloser(c.conn), nil
}

// StdoutPipe return the underlying connection as Reader
func (c *command) StdoutPipe() (io.Reader, error) {
	return c.conn, nil
}

func endpointToCommand(cmd string, ep transport.Endpoint) string {
	return fmt.Sprintf("%s %s%chost=%s%c", cmd, ep.Path, 0, ep.Host, 0)
}

// Wait no-op function, required by the interface
func (c *command) Wait() error {
	return nil
}

// Close closes the TCP connection and connection.
func (c *command) Close() error {
	if !c.connected {
		return nil
	}

	c.connected = false
	return c.conn.Close()
}
