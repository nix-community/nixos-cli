package utils

import (
	"fmt"
	"os"
	"os/exec"
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
