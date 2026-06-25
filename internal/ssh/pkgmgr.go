package ssh

import (
	"fmt"
	"strings"
)

// PackageManagerKind identifies which package manager a remote host uses.
type PackageManagerKind string

const (
	PkgMgrAPT     PackageManagerKind = "apt"    // Debian, Ubuntu, Mint, etc.
	PkgMgrDNF     PackageManagerKind = "dnf"    // Fedora, modern RHEL/CentOS/Rocky/Alma
	PkgMgrYum     PackageManagerKind = "yum"    // older RHEL/CentOS
	PkgMgrPacman  PackageManagerKind = "pacman" // Arch, Manjaro
	PkgMgrZypper  PackageManagerKind = "zypper" // openSUSE
	PkgMgrApk     PackageManagerKind = "apk"    // Alpine
	PkgMgrUnknown PackageManagerKind = ""
)

// PackageManager describes one detected package manager: which binary to
// invoke, and how to build its update/upgrade commands. Update/Upgrade
// build full shell command strings (the caller wraps them with sudo and
// DEBIAN_FRONTEND-equivalent noninteractive flags as needed per kind).
type PackageManager struct {
	Kind PackageManagerKind
	// DisplayName is shown in the UI so the person knows what's actually
	// running, e.g. "apt (Debian/Ubuntu)".
	DisplayName string
}

// knownPackageManagers lists candidates to probe for, in the order they're
// checked. Order matters where two could coexist (e.g. a RHEL system might
// have a leftover yum symlink to dnf) -- more specific/modern tools first.
var knownPackageManagers = []PackageManager{
	{Kind: PkgMgrAPT, DisplayName: "apt (Debian/Ubuntu)"},
	{Kind: PkgMgrDNF, DisplayName: "dnf (Fedora/RHEL/Rocky/Alma)"},
	{Kind: PkgMgrYum, DisplayName: "yum (older RHEL/CentOS)"},
	{Kind: PkgMgrPacman, DisplayName: "pacman (Arch/Manjaro)"},
	{Kind: PkgMgrZypper, DisplayName: "zypper (openSUSE)"},
	{Kind: PkgMgrApk, DisplayName: "apk (Alpine)"},
}

// DetectPackageManager probes the remote host for a known package manager
// binary and returns the first match. Detection is by binary presence
// (`command -v <name>`), not by parsing /etc/os-release: a distro's
// identity string is a less direct signal than "is the actual binary I'm
// about to call even there", and binary presence is what determines
// whether the resulting command can run at all.
func (c *Client) DetectPackageManager() (*PackageManager, error) {
	// One round-trip: ask the shell to print the first binary name found,
	// in priority order, rather than one SSH command per candidate.
	var names []string
	for _, pm := range knownPackageManagers {
		names = append(names, string(pm.Kind))
	}
	script := "for p in " + strings.Join(names, " ") + "; do command -v \"$p\" >/dev/null 2>&1 && echo \"$p\" && break; done"
	out, err := c.Run(script)
	if err != nil {
		return nil, fmt.Errorf("detect package manager: %w", err)
	}
	found := strings.TrimSpace(out)
	if found == "" {
		return nil, fmt.Errorf("no known package manager found on this host (checked: %s)", strings.Join(names, ", "))
	}
	for i := range knownPackageManagers {
		if string(knownPackageManagers[i].Kind) == found {
			pm := knownPackageManagers[i]
			return &pm, nil
		}
	}
	return nil, fmt.Errorf("detected unrecognized package manager binary: %s", found)
}

// updateCommand returns the shell command (without sudo/noninteractive
// wrapping) that refreshes the package index for this package manager.
func (pm PackageManager) updateCommand() string {
	switch pm.Kind {
	case PkgMgrAPT:
		return "apt-get update"
	case PkgMgrDNF:
		return "dnf check-update || true" // dnf has no separate "update index" step; check-update is the closest analog and exits 100 when updates exist, hence || true
	case PkgMgrYum:
		return "yum check-update || true"
	case PkgMgrPacman:
		return "pacman -Sy"
	case PkgMgrZypper:
		return "zypper refresh"
	case PkgMgrApk:
		return "apk update"
	default:
		return ""
	}
}

// upgradeCommand returns the shell command for a "safe" upgrade (one that
// won't remove packages or change major versions, matching the intent of
// apt's plain "upgrade" vs "dist-upgrade"). configAction is only
// meaningful for apt (dpkg config-file conflict policy); other package
// managers either don't have an equivalent interactive prompt in
// noninteractive mode or handle it differently and the argument is
// ignored for them.
func (pm PackageManager) upgradeCommand(configAction string, full bool) string {
	switch pm.Kind {
	case PkgMgrAPT:
		dpkgOpts := dpkgConfigOpts(configAction)
		verb := "upgrade"
		if full {
			verb = "dist-upgrade"
		}
		return fmt.Sprintf("%s apt-get %s -y", dpkgOpts, verb)
	case PkgMgrDNF:
		// dnf upgrade and dnf upgrade are the same; there's no separate
		// "safe vs full" distinction the way apt has upgrade/dist-upgrade,
		// since dnf already resolves dependency changes either way.
		return "dnf upgrade -y"
	case PkgMgrYum:
		return "yum update -y"
	case PkgMgrPacman:
		// pacman -Syu both refreshes the index and upgrades; -Su upgrades
		// using the already-refreshed local index. Since callers run
		// updateCommand separately first, -Su is the correct pairing here.
		return "pacman -Su --noconfirm"
	case PkgMgrZypper:
		verb := "update"
		if full {
			verb = "dist-upgrade"
		}
		return fmt.Sprintf("zypper --non-interactive %s", verb)
	case PkgMgrApk:
		return "apk upgrade"
	default:
		return ""
	}
}

func dpkgConfigOpts(configAction string) string {
	switch configAction {
	case "new":
		return `DPKG_OPTIONS='--force-confnew'`
	case "keep":
		return `DPKG_OPTIONS='--force-confold'`
	default:
		return `DPKG_OPTIONS='--force-confdef --force-confold'`
	}
}

// noninteractiveEnv returns the environment-variable prefix needed to keep
// this package manager from popping interactive prompts under a PTY.
func (pm PackageManager) noninteractiveEnv() string {
	switch pm.Kind {
	case PkgMgrAPT:
		return "DEBIAN_FRONTEND=noninteractive"
	default:
		// dnf/yum/zypper/apk are driven entirely by their own -y/
		// --non-interactive flags rather than an environment variable;
		// pacman's --noconfirm likewise needs no env var.
		return ""
	}
}
