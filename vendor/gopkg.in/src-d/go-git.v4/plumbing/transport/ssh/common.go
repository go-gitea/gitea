package ssh

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/internal/common"

	"golang.org/x/crypto/ssh"
)

var (
	errAlreadyConnected = errors.New("ssh session already created")
)

// DefaultClient is the default SSH client.
var DefaultClient = common.NewClient(&runner{})

type runner struct{}

func (r *runner) Command(cmd string, ep transport.Endpoint) (common.Command, error) {
	c := &command{command: cmd, endpoint: ep}
	if err := c.connect(); err != nil {
		return nil, err
	}

	return c, nil
}

type command struct {
	*ssh.Session
	connected bool
	command   string
	endpoint  transport.Endpoint
	client    *ssh.Client
	auth      AuthMethod
}

func (c *command) SetAuth(auth transport.AuthMethod) error {
	a, ok := auth.(AuthMethod)
	if !ok {
		return transport.ErrInvalidAuthMethod
	}

	c.auth = a
	return nil
}

func (c *command) Start() error {
	return c.Session.Start(endpointToCommand(c.command, c.endpoint))
}

// Close closes the SSH session and connection.
func (c *command) Close() error {
	if !c.connected {
		return nil
	}

	c.connected = false

	//XXX: If did read the full packfile, then the session might be already
	//     closed.
	_ = c.Session.Close()

	return c.client.Close()
}

// connect connects to the SSH server, unless a AuthMethod was set with
// SetAuth method, by default uses an auth method based on PublicKeysCallback,
// it connects to a SSH agent, using the address stored in the SSH_AUTH_SOCK
// environment var.
func (c *command) connect() error {
	if c.connected {
		return errAlreadyConnected
	}

	if err := c.setAuthFromEndpoint(); err != nil {
		return err
	}

	var err error
	c.client, err = ssh.Dial("tcp", c.getHostWithPort(), c.auth.clientConfig())
	if err != nil {
		return err
	}

	c.Session, err = c.client.NewSession()
	if err != nil {
		_ = c.client.Close()
		return err
	}

	c.connected = true
	return nil
}

func (c *command) getHostWithPort() string {
	host := c.endpoint.Host
	if strings.Index(c.endpoint.Host, ":") == -1 {
		host += ":22"
	}

	return host
}

func (c *command) setAuthFromEndpoint() error {
	var u string
	if info := c.endpoint.User; info != nil {
		u = info.Username()
	}

	var err error
	c.auth, err = NewSSHAgentAuth(u)
	return err
}

func endpointToCommand(cmd string, ep transport.Endpoint) string {
	return fmt.Sprintf("%s '%s'", cmd, ep.Path)
}
