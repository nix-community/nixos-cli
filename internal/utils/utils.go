package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
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

		defaultNixInfo, err := os.Stat(defaultNix)
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
