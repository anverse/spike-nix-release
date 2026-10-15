// Package sh runs command-line tools the way a shell script would: it
// prints each command, passes its output through, and returns an error
// when the command fails.
package sh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run runs a command and passes its output through.
func Run(name string, args ...string) error {
	return Cmd{}.Run(name, args...)
}

// Output runs a command and returns its output, trimmed.
func Output(name string, args ...string) (string, error) {
	return Cmd{}.Output(name, args...)
}

// Cmd holds what a command needs beyond its arguments.
type Cmd struct {
	Dir   string   // working directory; the current one if empty
	Env   []string // extra environment variables, "NAME=value"
	Stdin string   // text for the command's input
}

func (c Cmd) Run(name string, args ...string) error {
	fmt.Fprintln(os.Stderr, "+", name, strings.Join(args, " "))
	cmd := c.command(name, args)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func (c Cmd) Output(name string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := c.command(name, args)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func (c Cmd) command(name string, args []string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	return cmd
}
