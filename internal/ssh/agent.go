package ssh

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nix-community/nixos-cli/internal/logger"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// An SSH agent client manager. This will connect to an already-running
// SSH agent, or it will attempt to spin one up internally and set it
// as the process.
type AgentManager struct {
	// Connection to the SSH agent socket.
	agentConn net.Conn
	// The actual SSH agent client
	client agent.ExtendedAgent

	// Added keys should be removed from the SSH
	// agent if the server is not running.
	addedKeys []ssh.PublicKey

	// The server is only set and running if we are
	// creating our own in-memory SSH agent.
	server *agentServer

	logger logger.Logger
}

func NewAgentManager(log logger.Logger) (*AgentManager, error) {
	// Simply connect to existing agents if they are already there.
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		log.Debugf("connecting to existing SSH agent at %v", sock)
		dialer := net.Dialer{Timeout: 2 * time.Second}
		conn, err := dialer.Dial("unix", sock)
		if err != nil {
			conn.Close()
			return nil, err
		}

		client := agent.NewClient(conn)

		return &AgentManager{
			agentConn: conn,
			client:    client,
			logger:    log,
		}, nil
	}

	// Otherwise, create a new agent with a real socket path.
	// This will automatically set
	agentServer, err := newAgentServer(log)
	if err != nil {
		return nil, err
	}

	if err = agentServer.Start(); err != nil {
		_ = agentServer.Stop()
		return nil, err
	}

	// Once the server is up, the client is connected to it
	// automatically.
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.Dial("unix", agentServer.socketPath)
	if err != nil {
		_ = agentServer.Stop()
		return nil, err
	}

	client := agent.NewClient(conn)

	return &AgentManager{
		agentConn: conn,
		client:    client,
		server:    agentServer,
		logger:    log,
	}, nil
}

// Return the instantiated client.
func (m *AgentManager) Client() agent.ExtendedAgent {
	return m.client
}

// Add a key to the SSH agent.
//
// If it already exists in the agent, then it will not
// be added.
//
// For SSH servers connecting to an existing client,
// all explicitly added keys will be removed from the
// client when the manager is stopped.
func (m *AgentManager) Add(key any, comment string) error {
	m.logger.Debugf("adding SSH key to agent keyring")

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return err
	}

	publicKey := signer.PublicKey()

	if agentHasKey(m.client, publicKey) {
		return nil
	}

	err = m.client.Add(agent.AddedKey{
		PrivateKey: key,
		Comment:    comment,
	})
	if err != nil {
		return err
	}

	m.addedKeys = append(m.addedKeys, publicKey)

	return nil
}

func agentHasKey(client agent.ExtendedAgent, key ssh.PublicKey) bool {
	keys, err := client.List()
	if err != nil {
		return false
	}

	for _, k := range keys {
		var agentKey ssh.PublicKey
		agentKey, err = ssh.ParsePublicKey(k.Blob)
		if err != nil {
			continue
		}

		if bytes.Equal(agentKey.Marshal(), key.Marshal()) {
			return true
		}
	}

	return false
}

// Release any resources associated with the SSH agent client
// manager. This stops the internal agent server if it is
// running as well.
func (m *AgentManager) Stop() error {
	for _, key := range m.addedKeys {
		_ = m.client.Remove(key)
	}

	if m.server != nil {
		err := m.server.Stop()
		if err != nil {
			return err
		}
	}

	err := m.agentConn.Close()
	return err
}

// An in-memory SSH agent server that runs on a Unix socket.
type agentServer struct {
	listener   net.Listener
	keyring    agent.Agent
	socketPath string

	wg   sync.WaitGroup
	stop chan struct{}

	logger logger.Logger
}

func newAgentServer(log logger.Logger) (*agentServer, error) {
	socketPath := getSocketPath()
	_ = os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	keyring := agent.NewKeyring()

	return &agentServer{
		listener:   l,
		keyring:    keyring,
		stop:       make(chan struct{}),
		socketPath: socketPath,
		logger:     log,
	}, nil
}

// Start an in-memory SSH agent and set this agent for the process.
// Setting SSH_AUTH_SOCK is required for forked processes that use
// SSH agents, such as when copying Nix closures using the `nix`
// binary.
func (s *agentServer) Start() error {
	s.logger.Debugf("starting SSH agent server at %v", s.socketPath)
	err := os.Setenv("SSH_AUTH_SOCK", s.socketPath)
	if err != nil {
		return err
	}

	s.wg.Go(func() {
		for {
			s.accept()
		}
	})

	return nil
}

// Gracefully stop the SSH agent server and remove the socket. This also
// unsets SSH_AUTH_SOCK, assuming it has been set by the Start() routine
// beforehand.
func (s *agentServer) Stop() error {
	s.logger.Debugf("stopping SSH agent server")
	err := os.Unsetenv("SSH_AUTH_SOCK")
	if err != nil {
		return err
	}

	close(s.stop)
	s.listener.Close()
	s.wg.Wait()

	err = os.Remove(s.socketPath)
	return err
}

func (s *agentServer) accept() {
	conn, err := s.listener.Accept()
	if err != nil {
		select {
		case <-s.stop:
			return
		default:
			return
		}
	}

	s.wg.Add(1)
	go func(c net.Conn) {
		defer s.wg.Done()
		defer c.Close()
		if agentErr := agent.ServeAgent(s.keyring, c); agentErr != nil {
			s.logger.Errorf("agent server: %v", err)
		}
	}(conn)
}

func getSocketPath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("nixos-cli-%d.sock", os.Getpid()))
}
