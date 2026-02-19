package system

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	shlex "github.com/carapace-sh/carapace-shlex"
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	sshUtils "github.com/nix-community/nixos-cli/internal/ssh"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

type SSHSystem struct {
	conn       net.Conn
	client     *ssh.Client
	sftp       *sftp.Client
	user       string
	address    string
	port       int
	password   []byte
	keyFile    *TempFile
	nixSSHOpts []string
	logger     logger.Logger
}

func NewSSHSystem(localSystem *LocalSystem, host string, log logger.Logger, cfg *settings.Settings) (*SSHSystem, error) {
	if after, ok := strings.CutPrefix(host, "ssh://"); ok {
		host = after
	}

	hostInfo, err := sshUtils.ParseUserHostPort(host)
	if err != nil {
		return nil, err
	}

	var username string
	var address string
	var port int

	if hostInfo.User == "" {
		var current *user.User
		if current, err = user.Current(); err == nil {
			username = current.Username
		} else if current := os.Getenv("USER"); current != "" {
			username = current
		} else {
			return nil, fmt.Errorf("failed to determine current user")
		}
	} else {
		username = hostInfo.User
	}

	address = hostInfo.Host

	if hostInfo.Port != 0 {
		port = hostInfo.Port
	} else {
		port = 22
	}

	auth := []ssh.AuthMethod{}

	var keyFile *TempFile
	var nixSSHOpts []string
	if privateKeyCmd := cfg.SSH.PrivateKeyCmd; len(privateKeyCmd) > 0 {
		log.CmdArray(privateKeyCmd)

		var sshAuth ssh.AuthMethod
		sshAuth, keyFile, err = getPrivateKeyAuth(localSystem, host, username, privateKeyCmd)
		if err == nil {
			auth = append(auth, sshAuth)
			nixSSHOpts = append(nixSSHOpts, "-i", keyFile.Path())
		} else {
			log.Warnf("failed to obtain private key: %v", err)
		}
	}

	var conn net.Conn
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		dialer := net.Dialer{Timeout: 2 * time.Second}
		conn, err = dialer.Dial("unix", sock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			auth = append(auth, ssh.PublicKeysCallback(agentClient.Signers))
		} else {
			log.Warnf("failed to connect to SSH agent: %v", err)
			log.Warnf("falling back to password auth")
		}
	}

	getKnownHostsFiles := func(files []string, logFunc func(string, ...any)) (result []string) {
		for _, f := range files {
			if _, err = os.Stat(f); err != nil {
				logFunc("failed to access known hosts file: %v", err)
			} else {
				result = append(result, f)
			}
		}
		return result
	}
	knownHostsFileUser := filepath.Join(os.Getenv("HOME"), sshUtils.UserKnownHostsFile)
	knownHostsFilesBase := []string{sshUtils.SystemKnownHostsFile, knownHostsFileUser}
	knownHostsFiles := getKnownHostsFiles(knownHostsFilesBase, log.Debugf)
	knownHostsFiles = append(knownHostsFiles, getKnownHostsFiles(cfg.SSH.KnownHostsFiles, log.Warnf)...)
	knownHostsKeyCallback, err := knownhosts.New(knownHostsFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to create known hosts callback: %v", err)
	}

	var password []byte
	passwordCallback := ssh.PasswordCallback(func() (string, error) {
		var bytePassword []byte
		bytePassword, err = promptForPassword(username, address)
		if err != nil {
			return "", err
		}
		password = bytePassword
		return string(bytePassword), nil
	})
	auth = append(auth, passwordCallback)

	hostKeyCallback := wrappedKnownHostsCallback(log, knownHostsKeyCallback, knownHostsFileUser)

	client, err := ssh.Dial("tcp", net.JoinHostPort(address, strconv.Itoa(port)), &ssh.ClientConfig{
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
		conn:       conn,
		client:     client,
		sftp:       sftpClient,
		user:       username,
		address:    address,
		port:       port,
		password:   password,
		keyFile:    keyFile,
		nixSSHOpts: nixSSHOpts,
		logger:     log,
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

	if err = s.testRemoteRoot(rootCmd); err != nil {
		return fmt.Errorf("failed to verify %s password: %s", rootCmd, err)
	}

	return nil
}

func (s *SSHSystem) testRemoteRoot(rootCmd string) error {
	cmd := NewCommand("true").AsRoot(rootCmd)

	_, err := s.Run(cmd)
	return err
}

func getPrivateKeyAuth(s *LocalSystem, host string, username string, privateKeyCmd []string) (ssh.AuthMethod, *TempFile, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewCommand(privateKeyCmd[0], privateKeyCmd[1:]...)
	cmd.SetEnv("NIXOS_CLI_SSH_HOST", host)
	cmd.SetEnv("NIXOS_CLI_SSH_USER", username)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	var err error
	if _, err = s.Run(cmd); err != nil {
		return nil, nil, fmt.Errorf("failed to run private key command: %v\n%v", err, strings.TrimSpace(stderr.String()))
	}

	var signer ssh.Signer
	if signer, err = ssh.ParsePrivateKey(stdout.Bytes()); err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	keyFile, err := NewTempFile("nixos-cli-ssh-key", stdout.Bytes())
	if err != nil {
		return nil, nil, err
	}

	auth := ssh.PublicKeys(signer)

	return auth, keyFile, nil
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

				var confirm bool
				confirm, err = cmdUtils.ConfirmationInput("Are you sure you want to continue connecting?", cmdUtils.ConfirmationPromptOptions{
					// Copy the default SSH behavior of retrying for invalid input.
					// Disregard user configuration in this case, since this is mimicking
					// OpenSSH's behavior.
					InvalidBehavior: settings.ConfirmationPromptRetry,
					EmptyBehavior:   settings.ConfirmationPromptRetry,
				})
				if err != nil {
					log.Errorf("failed to get confirmation: %v", err)
					return err
				}
				if !confirm {
					return fmt.Errorf("user declined unknown host")
				}

				var knownHostsFile *os.File
				knownHostsFile, err = os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
				if err != nil {
					return fmt.Errorf("failed to open known_hosts: %w", err)
				}
				defer func() { _ = knownHostsFile.Close() }()

				line := knownhosts.Line([]string{hostname}, key)
				if _, err = knownHostsFile.WriteString(line + "\n"); err != nil {
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
		if closeErr := session.Close(); closeErr != nil && !errors.Is(closeErr, io.EOF) {
			log.Debugf("failed to close SSH session cleanly: %v", closeErr)
		}
	}()

	if cmd.RootElevationCmd != "" {
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
		if cmd.RootElevationCmd == "sudo" && s.password != nil {
			// Make a copy of the command struct to add the root elevation flags to
			cmd = cmd.Clone()
			cmd.RootElevationCmdFlags = append(cmd.RootElevationCmdFlags, "-S", "-p", "")
			pw := append([]byte{}, s.password...)
			pw = append(pw, '\n')
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
			var restoreLocal func()
			restoreLocal, err = requestRootPasswordPTY(session, cmd.Stdin)
			if err != nil {
				log.Warnf("unable to make local terminal raw: %v", err)
			} else {
				defer restoreLocal()
			}
		}
	}

	var cmdStr string
	if len(cmd.Env) > 0 {
		var args []string
		args, err = cmd.BuildShellWrapper()
		if err != nil {
			return 0, err
		}
		cmdStr = shlex.Join(args)
	} else {
		cmdStr = shlex.Join(cmd.BuildArgs())
	}

	session.Stdout = cmd.Stdout
	session.Stderr = cmd.Stderr

	// Forward stop signals to the remote process
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	go func() {
		for sig := range sigCh {
			if s := osSignalToSSHSignal(sig); s != "" {
				if err = session.Signal(s); err != nil {
					log.Warnf("failed to forward signal '%v': %v", s, err)
				}
			}
		}
	}()

	err = session.Run(cmdStr)
	if err == nil {
		return 0, nil
	}

	if exitErr, ok := err.(*ssh.ExitError); ok {
		return exitErr.ExitStatus(), err
	}

	return 0, err
}

func osSignalToSSHSignal(s os.Signal) ssh.Signal {
	switch s {
	case syscall.SIGABRT:
		return "ABRT"
	case syscall.SIGALRM:
		return "ALRM"
	case syscall.SIGFPE:
		return "FPE"
	case syscall.SIGHUP:
		return "HUP"
	case syscall.SIGILL:
		return "ILL"
	case syscall.SIGINT:
		return "INT"
	case syscall.SIGKILL:
		return "KILL"
	case syscall.SIGPIPE:
		return "PIPE"
	case syscall.SIGQUIT:
		return "QUIT"
	case syscall.SIGSEGV:
		return "SEGV"
	case syscall.SIGTERM:
		return "TERM"
	case syscall.SIGUSR1:
		return "USR1"
	case syscall.SIGUSR2:
		return "USR2"
	default:
		return ""
	}
}

func requestRootPasswordPTY(session *ssh.Session, stdin io.Reader) (func(), error) {
	file, ok := stdin.(*os.File)
	if !ok {
		return nil, errors.New("stdin is not a file")
	}

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

	if err = session.RequestPty(termType, h, w, modes); err != nil {
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
	return fmt.Sprintf("%s@%s:%d", s.user, s.address, s.port)
}

func (s *SSHSystem) NixSSHOpts() []string {
	return s.nixSSHOpts
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

	if s.keyFile != nil {
		_ = s.keyFile.Remove()
	}

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
