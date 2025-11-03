package system

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	shlex "github.com/carapace-sh/carapace-shlex"
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/utils"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

type SSHSystem struct {
	conn     net.Conn
	client   *ssh.Client
	sftp     *sftp.Client
	user     string
	address  string
	port     string
	password []byte
	logger   logger.Logger
}

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

	auth := []ssh.AuthMethod{}

	var conn net.Conn
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err = net.Dial("unix", sock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			auth = append(auth, ssh.PublicKeysCallback(agentClient.Signers))
		} else {
			log.Debug("failed to connect to SSH agent")
			log.Debug("falling back to password auth")
		}
	}

	knownHostsFile := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	knownHostsKeyCallback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create known hosts callback: %v", err)
	}

	var password []byte
	passwordCallback := ssh.PasswordCallback(func() (string, error) {
		bytePassword, err := promptForPassword(username, address)
		if err != nil {
			return "", err
		}
		password = bytePassword
		return string(bytePassword), nil
	})
	auth = append(auth, passwordCallback)

	hostKeyCallback := wrappedKnownHostsCallback(log, knownHostsKeyCallback, knownHostsFile)

	client, err := ssh.Dial("tcp", net.JoinHostPort(address, port), &ssh.ClientConfig{
		User:            username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
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
		conn:     conn,
		client:   client,
		sftp:     sftpClient,
		user:     username,
		address:  address,
		port:     port,
		password: password,
		logger:   log,
	}

	return s, nil
}

func (s *SSHSystem) EnsureRemoteRootPassword(rootCmd string) error {
	// If the password already exists, presumably we already have sudo
	// permissions and don't need to check. If the logged-in user doesn't,
	// then the first command that requires sudo will say as much and exit,
	// so no need to verify it explicitly here.
	if s.password != nil {
		return nil
	}

	s.Logger().Info("please input password to run commands as root")

	bytePassword, err := promptForPassword(s.user, s.address)
	if err != nil {
		return err
	}

	s.password = bytePassword

	if err := s.testRemoteRoot(rootCmd); err != nil {
		return fmt.Errorf("failed to verify %s password: %s", rootCmd, err)
	}

	return nil
}

func (s *SSHSystem) testRemoteRoot(rootCmd string) error {
	cmd := NewCommand("true").RunAsRoot(rootCmd)

	_, err := s.Run(cmd)
	return err
}

func promptForPassword(username string, address string) ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("cannot prompt for password: stdin is not a terminal")
	}

	fmt.Fprintf(os.Stderr, "Password for %s@%s: ", username, address)
	_ = os.Stdin.Sync()

	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return bytePassword, err
}

// This mimics the automatic addition of known_hosts entries
// to the known_hosts file that OpenSSH performs.
//
// Only occurs if the key is not already in known_hosts and
// if running in interactive mode in a terminal. Otherwise,
// this will result in failure to connect.
func wrappedKnownHostsCallback(log logger.Logger, origCallback ssh.HostKeyCallback, knownHostsPath string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := origCallback(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			// Only allow adding the key like OpenSSH does if the
			// stdin terminal can accept input.
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return err
			}

			if len(keyErr.Want) == 0 {
				fingerprint := ssh.FingerprintSHA256(key)
				log.Infof("the authenticity of host '%s' (%s) can't be established", hostname, key.Type())
				log.Infof("SHA256 fingerprint: %s", fingerprint)

				confirm, err := cmdUtils.ConfirmationInput("Are you sure you want to continue connecting (yes/no)?")
				if err != nil {
					log.Errorf("failed to get confirmation: %v", err)
					return err
				}
				if !confirm {
					return fmt.Errorf("user declined unknown host")
				}

				f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
				if err != nil {
					return fmt.Errorf("failed to open known_hosts: %w", err)
				}
				defer func() { _ = f.Close() }()

				line := knownhosts.Line([]string{hostname}, key)
				if _, err := f.WriteString(line + "\n"); err != nil {
					return fmt.Errorf("failed to write to known_hosts: %w", err)
				}

				log.Warnf("permanently added '%s' (%s) to the list of known hosts", hostname, key.Type())
				return nil
			}

			return fmt.Errorf("WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!\n"+
				"It is possible that someone is doing something nasty!\n"+
				"Offending key for host %s found in %s\n"+
				"Expected: %s\nGot: %s",
				hostname,
				knownHostsPath,
				ssh.FingerprintSHA256(keyErr.Want[0].Key),
				ssh.FingerprintSHA256(key),
			)
		}

		return err
	}
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

	if isRootCommand(cmd.Name) {
		// Pass `sudo` passwords from `stdin` if they are present.
		// This requires the `-S` flag.
		//
		// Processes will likely never expect stdin to be set for SSH
		// if they are running as root, since this seems to be a
		// fairly uncommon scenario to need to pass things through
		// stdin while simultaneously needing root, and we will likely
		// never need something like that here.
		//
		// As such, we're replacing the entire stdin with this password.
		if cmd.Name == "sudo" && s.password != nil {
			cmd.Args = append([]string{"-S", "-p", ""}, cmd.Args...)
			pw := append(s.password, '\n')
			session.Stdin = bytes.NewReader(pw)
		} else if isTerminal(cmd.Stdin) {
			session.Stdin = cmd.Stdin
			// sudo and other root-escalating commands need a PTY if running
			// in interactive mode.
			//
			// As such, we need to do the handling for this ourselves, which requires a few steps:
			//
			// 1. Request the PTY using the current terminal's size.
			// 2. Put the terminal into raw mode.
			// 3. Run the command.
			// 4. Restore terminal back to original state.
			//
			// The preferred way to avoid interactive input from root-escalating
			// commands is to have a user that can run such a command with a no-password
			// policy such as sudo's NOPASSWD directive, and to deploy using that.
			//
			// FIXME: Entering passwords interactively with `sudo` and a PTY
			// seems to have a bug where the first attempt is wrong due to
			// the PTY discarding the first inputted byte.
			restoreLocal, err := requestRootPasswordPTY(session, cmd.Stdin)

			if err != nil {
				log.Warnf("unable to make local terminal raw: %v", err)
			} else {
				defer restoreLocal()
			}
		}
	}

	argv := append([]string{cmd.Name}, cmd.Args...)
	fullCmd, err := buildSafeShellWrapper(argv, cmd.Env)
	if err != nil {
		return 0, err
	}

	session.Stdout = cmd.Stdout
	session.Stderr = cmd.Stderr

	// Forward stop signals to the remote process
	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	go func() {
		for {
			select {
			case sig := <-sigCh:
				_ = session.Signal(ssh.Signal(sig.String()))
			case <-done:
				return
			}
		}
	}()

	err = session.Run(fullCmd)
	close(done)
	if err == nil {
		return 0, nil
	}

	if exitErr, ok := err.(*ssh.ExitError); ok {
		return exitErr.ExitStatus(), err
	}

	return 0, err
}

func requestRootPasswordPTY(session *ssh.Session, stdin io.Reader) (func(), error) {
	file := stdin.(*os.File)
	w, h, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return nil, fmt.Errorf("failed to get terminal size: %w", err)
	}

	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "xterm"
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	if err := session.RequestPty(termType, h, w, modes); err != nil {
		return nil, fmt.Errorf("failed to allocate pty for process: %w", err)
	}

	fd := int(file.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}

	return func() {
		_ = term.Restore(fd, oldState)
	}, nil
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
	return fmt.Sprintf("%s@%s:%s", s.user, s.address, s.port)
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

	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func isTerminal(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}

	fd := file.Fd()
	return term.IsTerminal(int(fd))
}

func isRootCommand(cmd string) bool {
	switch cmd {
	case "sudo", "doas":
		return true
	default:
		return false
	}
}
