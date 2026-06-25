package ssh

import (
	"fmt"
	"io"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

// InteractiveSession wraps an SSH session with a PTY for interactive programs
// like apt, that may ask config questions or show progress.
type InteractiveSession struct {
	sess   *gossh.Session
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

// StartInteractive opens a PTY session and starts the given command.
// outputCb is called with each chunk of output as it arrives (for live streaming).
// inputCh can be used to send stdin lines (e.g. "y\n" for prompts).
// Package manager commands are run with their own noninteractive flags
// (DEBIAN_FRONTEND=noninteractive for apt, -y/--noconfirm for others; see
// pkgmgr.go), but a PTY is still requested for progress bars and color.
func (c *Client) StartInteractive(cmd string, outputCb func(string), inputCh <-chan string, doneCh chan<- error) error {
	sess, err := c.inner.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		return err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		sess.Close()
		return err
	}

	// Request a pseudo-terminal (needed for apt progress, colors, etc.)
	// Use 80x24 vt100 - screen readers don't care about terminal size
	modes := gossh.TerminalModes{
		gossh.ECHO:          0, // no echo (we show output ourselves)
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("vt100", 24, 200, modes); err != nil {
		// PTY not critical - fall back to non-PTY
		// apt will still work with DEBIAN_FRONTEND=noninteractive
	}

	is := &InteractiveSession{sess: sess, stdin: stdin, stdout: stdout, stderr: stderr}

	if err := sess.Start(cmd); err != nil {
		sess.Close()
		return fmt.Errorf("start: %w", err)
	}

	// Goroutine: stream stdout to callback
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				// Strip ANSI escape codes so screen reader gets clean text
				clean := stripANSI(string(buf[:n]))
				if clean != "" {
					outputCb(clean)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Goroutine: stream stderr to callback
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				clean := stripANSI(string(buf[:n]))
				if clean != "" {
					outputCb("[stderr] " + clean)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Goroutine: forward stdin from inputCh
	go func() {
		if inputCh == nil {
			return
		}
		for line := range inputCh {
			is.stdin.Write([]byte(line))
		}
	}()

	// Goroutine: wait for session to end
	go func() {
		err := sess.Wait()
		sess.Close()
		if doneCh != nil {
			doneCh <- err
		}
	}()

	return nil
}

// RunPackageUpdate refreshes the package index using whichever package
// manager pm represents (apt-get update, dnf check-update, pacman -Sy,
// etc.) -- see pkgmgr.go for the detection and per-manager command logic.
// Returns an error channel that fires when complete.
func (c *Client) RunPackageUpdate(pm PackageManager, outputCb func(string)) chan error {
	doneCh := make(chan error, 1)
	inner := pm.updateCommand()
	if inner == "" {
		doneCh <- fmt.Errorf("don't know how to update the package index for %s", pm.DisplayName)
		return doneCh
	}
	cmd := fmt.Sprintf(
		"echo %s | sudo -S bash -c %s",
		shellescape(c.SudoPass),
		shellescape(envPrefix(pm)+inner+" 2>&1"),
	)
	if err := c.StartInteractive(cmd, outputCb, nil, doneCh); err != nil {
		doneCh <- err
	}
	return doneCh
}

// RunPackageUpgrade runs a "safe" upgrade (no package removal, where the
// package manager distinguishes that) using whichever manager pm
// represents. configAction only affects apt's dpkg config-file conflict
// policy; it's a no-op for other package managers.
func (c *Client) RunPackageUpgrade(pm PackageManager, outputCb func(string), configAction string) chan error {
	return c.runPackageUpgrade(pm, outputCb, configAction, false)
}

// RunPackageFullUpgrade runs the "full" upgrade variant where the package
// manager has one (apt dist-upgrade, zypper dist-upgrade); for managers
// with no such distinction it's the same as RunPackageUpgrade.
func (c *Client) RunPackageFullUpgrade(pm PackageManager, outputCb func(string), configAction string) chan error {
	return c.runPackageUpgrade(pm, outputCb, configAction, true)
}

func (c *Client) runPackageUpgrade(pm PackageManager, outputCb func(string), configAction string, full bool) chan error {
	doneCh := make(chan error, 1)
	inner := pm.upgradeCommand(configAction, full)
	if inner == "" {
		doneCh <- fmt.Errorf("don't know how to upgrade packages for %s", pm.DisplayName)
		return doneCh
	}
	cmd := fmt.Sprintf(
		"echo %s | sudo -S bash -c %s",
		shellescape(c.SudoPass),
		shellescape(envPrefix(pm)+inner+" 2>&1"),
	)
	if err := c.StartInteractive(cmd, outputCb, nil, doneCh); err != nil {
		doneCh <- err
	}
	return doneCh
}

func envPrefix(pm PackageManager) string {
	env := pm.noninteractiveEnv()
	if env == "" {
		return ""
	}
	return env + " "
}

// RunInteractiveShell runs an arbitrary command with full PTY interaction.
// Used for the "open terminal" escape hatch when apt asks tricky questions.
// outputCb gets all output; inputCh feeds stdin; returned channel fires on exit.
func (c *Client) RunInteractiveShell(cmd string, outputCb func(string), inputCh <-chan string) chan error {
	doneCh := make(chan error, 1)
	if err := c.StartInteractive(cmd, outputCb, inputCh, doneCh); err != nil {
		doneCh <- err
	}
	return doneCh
}

// stripANSI removes ANSI escape sequences so screen readers get clean text
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip until letter (end of escape sequence)
			i += 2
			for i < len(s) {
				c := s[i]
				i++
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					break
				}
			}
			continue
		}
		// Skip other control chars except newline, tab, carriage return
		b := s[i]
		if b >= 0x20 || b == '\n' || b == '\t' || b == '\r' {
			out.WriteByte(b)
		}
		i++
	}
	return out.String()
}
