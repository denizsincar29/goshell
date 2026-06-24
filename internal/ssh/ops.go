package ssh

import (
	"fmt"
	"strconv"
	"strings"
)

// ---- Systemd ----

type ServiceStatus struct {
	Name        string
	Active      string // active, inactive, failed
	Sub         string // running, dead, exited
	Description string
	Enabled     string // enabled, disabled, static
}

// ListServices lists all systemd units of type service
func (c *Client) ListServices() ([]ServiceStatus, error) {
	out, err := c.Run("systemctl list-units --type=service --all --no-pager --no-legend 2>&1")
	if err != nil {
		return nil, fmt.Errorf("list-units: %w", err)
	}
	var svcs []ServiceStatus
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := strings.TrimPrefix(fields[0], "●")
		name = strings.TrimSpace(name)
		svc := ServiceStatus{
			Name:   name,
			Active: fields[2],
			Sub:    fields[3],
		}
		if len(fields) > 4 {
			svc.Description = strings.Join(fields[4:], " ")
		}
		svcs = append(svcs, svc)
	}
	return svcs, nil
}

// ServiceLogs returns recent journal logs for a service
func (c *Client) ServiceLogs(name string, lines int) (string, error) {
	cmd := fmt.Sprintf("journalctl -u %s -n %d --no-pager 2>&1", shellescape(name), lines)
	return c.Run(cmd)
}

// ServiceAction performs start/stop/restart/enable/disable on a service
func (c *Client) ServiceAction(name, action string, useSudo bool) (string, error) {
	cmd := fmt.Sprintf("systemctl %s %s", shellescape(action), shellescape(name))
	return c.RunMaybeSudo(cmd, useSudo)
}

// ServiceStatus returns detailed status for one service
func (c *Client) ServiceStatusDetail(name string) (string, error) {
	return c.Run(fmt.Sprintf("systemctl status %s --no-pager 2>&1", shellescape(name)))
}

// ---- Crontab ----

// GetCrontab returns the crontab for user (empty = current user)
func (c *Client) GetCrontab(user string) (string, error) {
	if user == "" {
		return c.Run("crontab -l 2>/dev/null || true")
	}
	return c.RunSudo(fmt.Sprintf("crontab -l -u %s 2>/dev/null || true", shellescape(user)))
}

// SetCrontab writes a new crontab for the user
func (c *Client) SetCrontab(user, content string, useSudo bool) (string, error) {
	escaped := strings.ReplaceAll(content, "'", "'\\''")
	var cmd string
	if user == "" {
		cmd = fmt.Sprintf("echo '%s' | crontab -", escaped)
	} else {
		cmd = fmt.Sprintf("echo '%s' | crontab -u %s -", escaped, shellescape(user))
	}
	return c.RunMaybeSudo(cmd, useSudo)
}

// ---- Filesystem ----

type FileEntry struct {
	Name        string
	IsDir       bool
	Size        int64
	Permissions string
	Owner       string
	Group       string
	Modified    string
}

// ListDir lists a directory with details
func (c *Client) ListDir(path string) ([]FileEntry, error) {
	// Use stat to get machine-readable output
	cmd := fmt.Sprintf("ls -la --time-style=long-iso %s 2>&1", shellescape(path))
	out, err := c.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("ls %s: %w: %s", path, err, out)
	}
	var entries []FileEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		perms := fields[0]
		owner := fields[2]
		group := fields[3]
		sizeStr := fields[4]
		modified := fields[5] + " " + fields[6]
		name := strings.Join(fields[8:], " ")
		if name == "." || name == ".." {
			continue
		}
		size, _ := strconv.ParseInt(sizeStr, 10, 64)
		entries = append(entries, FileEntry{
			Name:        name,
			IsDir:       strings.HasPrefix(perms, "d"),
			Size:        size,
			Permissions: perms,
			Owner:       owner,
			Group:       group,
			Modified:    modified,
		})
	}
	return entries, nil
}

// ReadFile reads a remote file as UTF-8 string. Uses sudo if requested.
func (c *Client) ReadFile(path string, useSudo bool) (string, error) {
	// Use cat with explicit encoding handling
	cmd := fmt.Sprintf("cat %s", shellescape(path))
	out, err := c.RunMaybeSudo(cmd, useSudo)
	if err != nil {
		return "", fmt.Errorf("read %s: %w: %s", path, err, out)
	}
	return out, nil
}

// WriteFile writes content to a remote file atomically via temp file
// Preserves original permissions. Handles encoding.
func (c *Client) WriteFile(path, content string, useSudo bool) error {
	// Write to a temp file first, then move atomically
	tmpPath := fmt.Sprintf("/tmp/goshell_edit_%d", uniqueID())
	// Escape content for here-doc (avoid issues with special chars)
	// Use base64 encoding to be completely safe with binary/special chars
	encoded := b64Encode([]byte(content))
	writeCmd := fmt.Sprintf("echo %s | base64 -d > %s", shellescape(encoded), shellescape(tmpPath))
	if _, err := c.RunMaybeSudo(writeCmd, useSudo); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	// Copy permissions from original if it exists
	chmodCmd := fmt.Sprintf(
		"if [ -f %s ]; then chmod --reference=%s %s; fi && mv %s %s",
		shellescape(path), shellescape(path), shellescape(tmpPath),
		shellescape(tmpPath), shellescape(path),
	)
	if _, err := c.RunMaybeSudo(chmodCmd, useSudo); err != nil {
		c.RunMaybeSudo("rm -f "+shellescape(tmpPath), useSudo) // cleanup
		return fmt.Errorf("move: %w", err)
	}
	return nil
}

// Chmod changes permissions on a file/dir
func (c *Client) Chmod(path, mode string, recursive bool, useSudo bool) (string, error) {
	r := ""
	if recursive {
		r = "-R "
	}
	cmd := fmt.Sprintf("chmod %s%s %s", r, shellescape(mode), shellescape(path))
	return c.RunMaybeSudo(cmd, useSudo)
}

// Chown changes owner/group on a file/dir
func (c *Client) Chown(path, owner, group string, recursive bool, useSudo bool) (string, error) {
	ownerGroup := shellescape(owner)
	if group != "" {
		ownerGroup = shellescape(owner + ":" + group)
	}
	r := ""
	if recursive {
		r = "-R "
	}
	cmd := fmt.Sprintf("chown %s%s %s", r, ownerGroup, shellescape(path))
	return c.RunMaybeSudo(cmd, useSudo)
}

// DiskInfo is one row of `df -h` output, broken into actual fields.
type DiskInfo struct {
	Filesystem string `json:"filesystem"`
	Size       string `json:"size"`
	Used       string `json:"used"`
	Avail      string `json:"avail"`
	UsePercent string `json:"use_percent"`
	MountedOn  string `json:"mounted_on"`
}

// DiskUsage returns disk usage info as structured rows.
func (c *Client) DiskUsage() ([]DiskInfo, error) {
	out, err := c.Run("df -h 2>&1")
	if err != nil {
		return nil, fmt.Errorf("df -h: %w: %s", err, out)
	}
	return parseDFH(out), nil
}

func parseDFH(out string) []DiskInfo {
	lines := strings.Split(out, "\n")
	var disks []DiskInfo
	for i, line := range lines {
		if i == 0 {
			continue // header row (Filesystem Size Used Avail Use% Mounted on)
		}
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		disks = append(disks, DiskInfo{
			Filesystem: fields[0],
			Size:       fields[1],
			Used:       fields[2],
			Avail:      fields[3],
			UsePercent: fields[4],
			MountedOn:  strings.Join(fields[5:], " "),
		})
	}
	return disks
}

// DirSize returns size of a specific directory
func (c *Client) DirSize(path string, useSudo bool) (string, error) {
	cmd := fmt.Sprintf("du -sh %s 2>&1", shellescape(path))
	return c.RunMaybeSudo(cmd, useSudo)
}

// ---- Resources ----

type ResourceInfo struct {
	CPUPercent float64
	MemTotal   int64
	MemUsed    int64
	MemFree    int64
	SwapTotal  int64
	SwapUsed   int64
	LoadAvg    string
	Uptime     string
}

// GetResources returns current CPU/RAM/swap stats
func (c *Client) GetResources() (*ResourceInfo, error) {
	out, err := c.Run(`
		echo "=UPTIME=" && uptime
		echo "=CPU=" && top -bn1 | grep "Cpu(s)" | awk '{print $2+$4}'
		echo "=MEM=" && free -b | awk 'NR==2{print $2,$3,$4}'
		echo "=SWAP=" && free -b | awk 'NR==3{print $2,$3}'
	`)
	if err != nil {
		return nil, err
	}
	ri := &ResourceInfo{}
	sections := parseSections(out)
	if v, ok := sections["UPTIME"]; ok {
		ri.Uptime = strings.TrimSpace(v)
	}
	if v, ok := sections["CPU"]; ok {
		fmt.Sscanf(strings.TrimSpace(v), "%f", &ri.CPUPercent)
	}
	if v, ok := sections["MEM"]; ok {
		fmt.Sscanf(strings.TrimSpace(v), "%d %d %d", &ri.MemTotal, &ri.MemUsed, &ri.MemFree)
	}
	if v, ok := sections["SWAP"]; ok {
		fmt.Sscanf(strings.TrimSpace(v), "%d %d", &ri.SwapTotal, &ri.SwapUsed)
	}
	return ri, nil
}

// ProcessInfo is one row of `ps aux` output, broken into actual fields
// instead of one fixed-width text line. ps aux's first 10 columns are
// single whitespace-delimited tokens; everything after that is COMMAND,
// which may itself contain spaces, so it's captured as the remainder.
type ProcessInfo struct {
	User    string  `json:"user"`
	PID     string  `json:"pid"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	VSZ     string  `json:"vsz"`
	RSS     string  `json:"rss"`
	TTY     string  `json:"tty"`
	Stat    string  `json:"stat"`
	Start   string  `json:"start"`
	Time    string  `json:"time"`
	Command string  `json:"command"`
}

// GetProcesses returns the top processes by CPU usage as structured rows.
func (c *Client) GetProcesses() ([]ProcessInfo, error) {
	out, err := c.Run("ps aux --sort=-%cpu 2>&1 | head -31") // header + 30 rows
	if err != nil {
		return nil, fmt.Errorf("ps aux: %w: %s", err, out)
	}
	return parsePSAux(out), nil
}

func parsePSAux(out string) []ProcessInfo {
	lines := strings.Split(out, "\n")
	var procs []ProcessInfo
	for i, line := range lines {
		if i == 0 {
			continue // header row (USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND)
		}
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		mem, _ := strconv.ParseFloat(fields[3], 64)
		procs = append(procs, ProcessInfo{
			User:    fields[0],
			PID:     fields[1],
			CPU:     cpu,
			Mem:     mem,
			VSZ:     fields[4],
			RSS:     fields[5],
			TTY:     fields[6],
			Stat:    fields[7],
			Start:   fields[8],
			Time:    fields[9],
			Command: strings.Join(fields[10:], " "),
		})
	}
	return procs
}

// ---- Helper installation ----

// InstallHelper installs the goshell-helper binary on the remote server
// The helper provides atomic operations and better resource info
func (c *Client) InstallHelper(helperBinary []byte) error {
	remotePath := "/usr/local/bin/goshell-helper"
	encoded := b64Encode(helperBinary)
	// Write in chunks to avoid ARG_MAX
	chunkSize := 60000
	// First clear the file
	if _, err := c.RunSudo(fmt.Sprintf("rm -f %s && touch %s", remotePath, remotePath)); err != nil {
		return err
	}
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		cmd := fmt.Sprintf("echo -n %s >> /tmp/goshell_helper_b64", shellescape(chunk))
		if _, err := c.Run(cmd); err != nil {
			return fmt.Errorf("upload chunk: %w", err)
		}
	}
	installCmd := fmt.Sprintf(
		"base64 -d /tmp/goshell_helper_b64 > %s && chmod +x %s && rm /tmp/goshell_helper_b64",
		remotePath, remotePath,
	)
	if _, err := c.RunSudo(installCmd); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	return nil
}

// ---- Utilities ----

func parseSections(out string) map[string]string {
	result := make(map[string]string)
	var current string
	var sb strings.Builder
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "=") && strings.HasSuffix(strings.TrimSpace(line), "=") {
			if current != "" {
				result[current] = sb.String()
				sb.Reset()
			}
			current = strings.Trim(strings.TrimSpace(line), "=")
		} else if current != "" {
			sb.WriteString(line + "\n")
		}
	}
	if current != "" {
		result[current] = sb.String()
	}
	return result
}

var idCounter int64

func uniqueID() int64 {
	idCounter++
	return idCounter
}

func b64Encode(data []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var sb strings.Builder
	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		sb.WriteByte(chars[b0>>2])
		sb.WriteByte(chars[((b0&3)<<4)|(b1>>4)])
		if i+1 < len(data) {
			sb.WriteByte(chars[((b1&15)<<2)|(b2>>6)])
		} else {
			sb.WriteByte('=')
		}
		if i+2 < len(data) {
			sb.WriteByte(chars[b2&63])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}
