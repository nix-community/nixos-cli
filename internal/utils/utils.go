package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// Re-exec the current process as root with the same arguments.
// This is done with the provided rootCommand parameter, which
// usually is "sudo" or "doas", and comes from the command config.
func ExecAsRoot(rootCommand string) error {
	rootCommandPath, err := exec.LookPath(rootCommand)
	if err != nil {
		return err
	}

	argv := []string{rootCommand}
	argv = append(argv, os.Args...)

	err = syscall.Exec(rootCommandPath, argv, os.Environ())
	return err
}

func EscapeAndJoinArgs(args []string) string {
	var escapedArgs []string

	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"'\\") {
			arg = strings.ReplaceAll(arg, "\\", "\\\\")
			arg = strings.ReplaceAll(arg, "\"", "\\\"")
			escapedArgs = append(escapedArgs, fmt.Sprintf("\"%s\"", arg))
		} else {
			escapedArgs = append(escapedArgs, arg)
		}
	}

	return strings.Join(escapedArgs, " ")
}

var specialCharsPattern = regexp.MustCompile(`[^\w@%+=:,./-]`)

// Quote returns a shell-escaped version of the string s. The returned value
// is a string that can safely be used as one token in a shell command line.
//
// Taken directly from github.com/alessio/shellescape.
func Quote(s string) string {
	if len(s) == 0 {
		return "''"
	}

	if specialCharsPattern.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}

	return s
}

// Resolve a Nix filename to a real file.
//
// If `filename` is a file, then make sure it exists.
//
// If it is a directory, then append "default.nix" to
// it and then make sure that file exists.
//
// A stat error will be returned for the file that is supposed
// to exist if it does not.
func ResolveNixFilename(input string) (string, error) {
	fileInfo, err := os.Stat(input)
	if err != nil {
		return "", err
	}

	var resolved string

	if !fileInfo.IsDir() {
		resolved = input
	} else {
		defaultNix := filepath.Join(input, "default.nix")

		var defaultNixInfo os.FileInfo
		defaultNixInfo, err = os.Stat(defaultNix)
		if err != nil {
			return "", err
		}

		if defaultNixInfo.IsDir() {
			return "", fmt.Errorf("%v is a directory, not a file", defaultNix)
		}

		resolved = defaultNix
	}

	// Nix does not work well with relative addressing, so
	// make sure to resolve it to an absolute, canonical
	// path preemptively.
	realPath, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", err
	}

	absolutePath, err := filepath.Abs(realPath)
	if err != nil {
		return "", err
	}

	return absolutePath, nil
}

func ResolveDirectory(input string) (string, error) {
	fileInfo, err := os.Stat(input)
	if err != nil {
		return "", err
	} else if !fileInfo.IsDir() {
		return "", fmt.Errorf("not a directory: %v", input)
	}

	realPath, err := filepath.EvalSymlinks(input)
	if err != nil {
		return "", err
	}

	absolutePath, err := filepath.Abs(realPath)
	if err != nil {
		return "", err
	}

	return absolutePath, nil
}

// Prompt for a password from /dev/tty or stdin.
//
// This operation can be cancelled using the provided context,
// but any errors returned from here MAY have the potential
// to keep consuming stdin until another character is typed
// if /dev/tty or a duplicate instance of stdin cannot be
// opened, so any errors here will result in potentially
// undefined behavior for stdin input.
func PromptForPassword(ctx context.Context, prompt string) ([]byte, error) {
	var fd int

	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer tty.Close()
		fd = int(tty.Fd())
	} else if term.IsTerminal(int(os.Stdin.Fd())) {
		dupStdin, openErr := os.OpenFile("/dev/stdin", os.O_RDONLY, 0)
		if openErr == nil {
			defer dupStdin.Close()
			fd = int(dupStdin.Fd())
		} else {
			// NOTE: falling back to stdin will make context
			// cancellation behavior a bit unclear, as mentioned
			// in the doc comment.
			fd = int(os.Stdin.Fd())
		}
	} else {
		return nil, fmt.Errorf("standard input is not a terminal, and /dev/tty is not available: %v", err)
	}

	oldState, err := term.GetState(fd)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "%s", prompt)

	type result struct {
		pw  []byte
		err error
	}

	ch := make(chan result, 1)

	go func() {
		pw, readErr := term.ReadPassword(fd)
		ch <- result{pw, readErr}
	}()

	select {
	case <-ctx.Done():
		_ = term.Restore(fd, oldState)
		fmt.Fprintln(os.Stderr)
		return nil, ctx.Err()

	case res := <-ch:
		fmt.Fprintln(os.Stderr)
		return res.pw, res.err
	}
}

// Get the current user's username.
func GetUsername() (string, error) {
	if current, err := user.Current(); err == nil {
		return current.Username, nil
	} else if u := os.Getenv("USER"); u != "" {
		return u, nil
	} else {
		return "", fmt.Errorf("failed to determine current user")
	}
}

// Find if a directory contains a file.
func ContainsFile(dir string, filename string) bool {
	s, err := os.Stat(dir)
	if err != nil {
		return false
	}

	if !s.IsDir() {
		return false
	}

	if _, err = os.Stat(filepath.Join(dir, filename)); err != nil {
		return false
	}

	return true
}
