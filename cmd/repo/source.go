package repo

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"
)

// Clone protocols accepted by --protocol.
const (
	protocolSSH   = "ssh"
	protocolHTTPS = "https"
)

// resolveSource turns --source into a repos.yaml source. protocol is --protocol:
// empty when not given. An org/repo shorthand becomes a GitHub URL in protocol,
// ssh by default. A full URL is kept as written, and a local path too; giving
// --protocol for either is accepted only when it agrees with the URL, so a
// contradiction is reported rather than one of the two silently winning.
func resolveSource(source, protocol string, canPrompt bool, reader *bufio.Reader) (string, error) {
	source = strings.TrimSpace(source)
	if protocol != "" && protocol != protocolSSH && protocol != protocolHTTPS {
		return "", fmt.Errorf("--protocol must be ssh or https")
	}

	// If missing, go interactive when prompts are allowed.
	if source == "" {
		if !canPrompt {
			return "", fmt.Errorf("missing required value(s): --source")
		}
		v := promptSourceInteractive(reader, protocol)
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("aborted")
		}
		return v, checkProtocol(v, protocol)
	}

	// URL forms (HTTPS/SSH) are accepted as-is.
	if isURL(source) {
		return source, checkProtocol(source, protocol)
	}

	// Local path is accepted as-is.
	if isLocalPath(source) {
		return source, checkProtocol(source, protocol)
	}

	// org/repo: a GitHub repository, cloned over ssh unless --protocol says
	// otherwise.
	if strings.Count(source, "/") == 1 {
		if protocol == "" {
			protocol = protocolSSH
		}
		return formatGitHubSource(protocol, strings.TrimSuffix(source, ".git")), nil
	}

	// We intentionally do not support org-only here to keep behavior simple.
	// Use a full URL or org/repo.
	if strings.Count(source, "/") == 0 {
		return "", fmt.Errorf("invalid --source %q: expected URL, org/repo, or local path", source)
	}

	// Unknown; return as-is.
	return source, nil
}

func isURL(source string) bool {
	return strings.Contains(source, "://") || strings.Contains(source, "@")
}

func isLocalPath(source string) bool {
	return strings.HasPrefix(source, "/") || strings.HasPrefix(source, "~")
}

// urlProtocol returns the protocol a source clones with: ssh for ssh:// and
// scp-like (git@host:org/repo) URLs, https for http(s):// URLs, and "" for
// anything else (a local path, file://, git://).
func urlProtocol(source string) string {
	switch {
	case strings.HasPrefix(source, "https://"), strings.HasPrefix(source, "http://"):
		return protocolHTTPS
	case strings.HasPrefix(source, "ssh://"):
		return protocolSSH
	case !strings.Contains(source, "://") && strings.Contains(source, "@"):
		return protocolSSH
	}
	return ""
}

// checkProtocol reports a --protocol that contradicts the source it came with.
func checkProtocol(source, protocol string) error {
	if protocol == "" {
		return nil
	}
	got := urlProtocol(source)
	if got == "" {
		return fmt.Errorf("--protocol %s does not apply to --source %q, which is not an ssh or https URL", protocol, source)
	}
	if got != protocol {
		return fmt.Errorf("--protocol %s contradicts --source %q, which is an %s URL", protocol, source, got)
	}
	return nil
}

func promptSourceInteractive(reader *bufio.Reader, protocol string) string {
	kind, err := promptSelect(reader, "Source kind", []string{"GitHub (URL or org/repo)", "Local path"}, 10, false)
	if err != nil {
		return ""
	}
	switch kind {
	case 0:
		v := promptRequired(reader, "GitHub repo (URL or org/repo)", "")
		v = strings.TrimSpace(v)
		// Allow URL forms (HTTPS/SSH) as-is.
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.Contains(v, "://") || strings.Contains(v, "@") {
			return v
		}

		// org/repo (or org/repo.git): ask SSH vs HTTPS unless --protocol
		// already said.
		if strings.Count(v, "/") == 1 && !isLocalPath(v) {
			orgRepo := strings.TrimSuffix(v, ".git")
			proto := protocol
			if proto == "" {
				proto = promptGitHubProtocol(reader)
			}
			return formatGitHubSource(proto, orgRepo)
		}
		return v
	case 1:
		return promptRequired(reader, "Local path", "")
	default:
		return promptRequired(reader, "Source URL or local path", "")
	}
}

func promptGitHubProtocol(reader *bufio.Reader) string {
	i, err := promptSelect(reader, "Protocol", []string{"SSH", "HTTPS"}, 10, false)
	if err != nil {
		return "https"
	}
	if i == 0 {
		return "ssh"
	}
	return "https"
}

func formatGitHubSource(proto, orgRepo string) string {
	orgRepo = strings.TrimSpace(strings.TrimSuffix(orgRepo, ".git"))
	switch proto {
	case protocolSSH:
		return "git@github.com:" + orgRepo + ".git"
	default:
		return "https://github.com/" + orgRepo + ".git"
	}
}

func inferNameFromSource(source string) string {
	s := strings.TrimSpace(source)
	if s == "" {
		return ""
	}

	// Local path: use last path element.
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~") {
		base := filepath.Base(s)
		if base == "." || base == "/" || base == "~" {
			return ""
		}
		return base
	}

	// Remote: try to extract the final path segment and strip ".git".
	lastSlash := strings.LastIndex(s, "/")
	lastColon := strings.LastIndex(s, ":")
	lastSep := lastSlash
	if lastColon > lastSep {
		lastSep = lastColon
	}
	if lastSep >= 0 && lastSep+1 < len(s) {
		s = s[lastSep+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return s
}
