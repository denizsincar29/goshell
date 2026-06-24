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
// Use DEBIAN_FRONTEND=noninteractive to suppress most apt dialogs,
// but we still need a PTY for progress bars.
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

// RunAptUpdate runs apt-get update with live output streaming.
// Returns error channel that fires when complete.
func (c *Client) RunAptUpdate(outputCb func(string)) chan error {
	doneCh := make(chan error, 1)
	cmd := fmt.Sprintf(
		"echo %s | sudo -S bash -c 'DEBIAN_FRONTEND=noninteractive apt-get update 2>&1'",
		shellescape(c.SudoPass),
	)
	if err := c.StartInteractive(cmd, outputCb, nil, doneCh); err != nil {
		doneCh <- err
	}
	return doneCh
}

// RunAptUpgrade runs apt-get upgrade with live output.
// For config prompts (dpkg asking about config files), we pass --keep-old-files
// to avoid interactive dialogs. User can choose behavior.
func (c *Client) RunAptUpgrade(outputCb func(string), configAction string) chan error {
	doneCh := make(chan error, 1)
	// configAction: "keep" = keep old configs, "new" = use new, "ask" = show terminal
	var dpkgOpts string
	switch configAction {
	case "new":
		dpkgOpts = `DPKG_OPTIONS='--force-confnew'`
	case "keep":
		dpkgOpts = `DPKG_OPTIONS='--force-confold'`
	default: // "ask" - use noninteractive with default (keep)
		dpkgOpts = `DPKG_OPTIONS='--force-confdef --force-confold'`
	}

	cmd := fmt.Sprintf(
		"echo %s | sudo -S bash -c 'DEBIAN_FRONTEND=noninteractive %s apt-get upgrade -y 2>&1'",
		shellescape(c.SudoPass),
		dpkgOpts,
	)
	if err := c.StartInteractive(cmd, outputCb, nil, doneCh); err != nil {
		doneCh <- err
	}
	return doneCh
}

// RunAptDistUpgrade runs a full dist-upgrade
func (c *Client) RunAptDistUpgrade(outputCb func(string), configAction string) chan error {
	doneCh := make(chan error, 1)
	var dpkgOpts string
	switch configAction {
	case "new":
		dpkgOpts = `DPKG_OPTIONS='--force-confnew'`
	default:
		dpkgOpts = `DPKG_OPTIONS='--force-confdef --force-confold'`
	}
	cmd := fmt.Sprintf(
		"echo %s | sudo -S bash -c 'DEBIAN_FRONTEND=noninteractive %s apt-get dist-upgrade -y 2>&1'",
		shellescape(c.SudoPass),
		dpkgOpts,
	)
	if err := c.StartInteractive(cmd, outputCb, nil, doneCh); err != nil {
		doneCh <- err
	}
	return doneCh
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
