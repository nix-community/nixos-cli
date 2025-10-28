package system

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	shlex "github.com/carapace-sh/carapace-shlex"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/utils"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHSystem struct {
	conn   net.Conn
	client *ssh.Client
	sftp   *sftp.Client
	user   string
	host   string
	port   string
	logger logger.Logger
}

var ErrAgentNotStarted = fmt.Errorf("SSH_AUTH_SOCK not set; please start or forward an SSH agent")

func NewSSHSystem(host string, log logger.Logger) (*SSHSystem, error) {
	if host == "" {
		return nil, fmt.Errorf("host string is empty")
	}

	if !strings.Contains(host, "://") {
		host = "ssh://" + host
	}

	parsedURL, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host: %v", err)
	}

	var username string
	if u := parsedURL.User; u != nil {
		username = u.Username()
	} else {
		current, err := user.Current()
		if err != nil {
			username = os.Getenv("USER")
			if username == "" {
				return nil, fmt.Errorf("failed to determine current user: %w", err)
			}
		} else {
			username = current.Username
		}
	}

	address := parsedURL.Hostname()

	var port string
	if p := parsedURL.Port(); p != "" {
		port = p
	} else {
		port = "22"
	}

	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSock == "" {
		return nil, ErrAgentNotStarted
	}

	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH socket: %s", err)
	}
	agentClient := agent.NewClient(conn)

	auth := []ssh.AuthMethod{ssh.PublicKeysCallback(agentClient.Signers)}

	knownHostsKeyCallback, err := knownhosts.New(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("failed to create known hosts callback: %v", err)
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(address, port), &ssh.ClientConfig{
		User:            username,
		Auth:            auth,
		HostKeyCallback: knownHostsKeyCallback,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", host, err)
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to instantiate SFTP client: %w", err)
	}

	s := &SSHSystem{
		conn:   conn,
		client: client,
		sftp:   sftpClient,
		user:   username,
		host:   address,
		port:   port,
		logger: log,
	}

	return s, nil
}

func (s *SSHSystem) FS() Filesystem {
	return &SFTPFilesystem{client: s.sftp}
}

func (s *SSHSystem) Logger() logger.Logger {
	return s.logger
}

func (s *SSHSystem) Run(cmd *Command) (int, error) {
	log := s.logger

	session, err := s.client.NewSession()
	if err != nil {
		return 0, fmt.Errorf("failed to create SSH session: %w", err)
	}

	defer func() {
		if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
			log.Debugf("failed to close SSH session cleanly: %v", err)
		}
	}()

	session.Stdin = cmd.Stdin
	session.Stdout = cmd.Stdout
	session.Stderr = cmd.Stderr

	argv := append([]string{cmd.Name}, cmd.Args...)
	fullCmd, err := buildSafeShellWrapper(argv, cmd.Env)
	if err != nil {
		return 0, err
	}

	err = session.Run(fullCmd)
	if err == nil {
		return 0, nil
	}

	if exitErr, ok := err.(*ssh.ExitError); ok {
		return exitErr.ExitStatus(), err
	}

	return 0, err
}

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Build a safe `sh -c` invocation that can support setting
// environment variables for a process, even without the proper
// AcceptEnv settings existing on the SSH system.
//
// Creates a string with the contents:
// `sh -c 'export KEY='val'; export KEY2='val2'; set -- 'arg0' 'arg1'; exec "$@"'`
func buildSafeShellWrapper(argv []string, env map[string]string) (string, error) {
	if len(argv) == 0 {
		return "", errors.New("argv must contain at least one element")
	}

	for k, v := range env {
		if !envVarNamePattern.MatchString(k) {
			return "", errors.New("invalid env var name: " + k)
		}
		if strings.IndexByte(v, 0) != -1 {
			return "", errors.New("NUL (0x00) bytes are not allowed in env values or args")
		}
	}
	for _, a := range argv {
		if strings.IndexByte(a, 0) != -1 {
			return "", errors.New("NUL (0x00) bytes are not allowed in env values or args")
		}
	}

	// deterministic ordering for env exports
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		q := utils.Quote(env[k])
		b.WriteString("export ")
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(q)
		b.WriteString("; ")
	}

	// set positional parameters
	b.WriteString("set --")
	for _, a := range argv {
		q := utils.Quote(a)
		b.WriteByte(' ')
		b.WriteString(q)
	}
	b.WriteString("; exec \"$@\"")

	snippet := b.String()
	return fmt.Sprintf("sh -c %v", utils.Quote(snippet)), nil
}

func (s *SSHSystem) IsNixOS() bool {
	_, err := s.sftp.Stat("/etc/NIXOS")
	if err == nil {
		return true
	}

	osReleaseFile, err := s.sftp.Open("/etc/os-release")
	if err != nil {
		return false
	}
	defer func() { _ = osReleaseFile.Close() }()

	osRelease, err := parseOSRelease(osReleaseFile)
	if err != nil {
		return false
	}

	distroID, ok := osRelease["ID"]
	if !ok {
		return false
	}

	return nixosDistroIDRegex.MatchString(distroID)
}

func (s *SSHSystem) Address() string {
	return fmt.Sprintf("%s@%s:%s", s.user, s.host, s.port)
}

func (s *SSHSystem) IsRemote() bool {
	return true
}

func (s *SSHSystem) HasCommand(name string) bool {
	session, err := s.client.NewSession()
	if err != nil {
		return false
	}
	defer func() { _ = session.Close() }()

	session.Stdout = nil
	session.Stderr = nil

	cmdStr := shlex.Join([]string{"command", "-v", name})

	err = session.Run(cmdStr)
	return err == nil
}

func (s *SSHSystem) Close() {
	_ = s.sftp.Close()
	_ = s.client.Close()
	_ = s.conn.Close()
}
