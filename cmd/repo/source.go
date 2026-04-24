package repo

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"
)

func resolveSource(source string, isTTY bool, reader *bufio.Reader) (string, error) {
	source = strings.TrimSpace(source)

	// If missing, go interactive (TTY only).
	if source == "" {
		if !isTTY {
			return "", fmt.Errorf("missing required value(s): --source")
		}
		v := promptSourceInteractive(reader)
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("aborted")
		}
		return v, nil
	}

	// URL forms (HTTPS/SSH) are accepted as-is.
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") || strings.Contains(source, "://") || strings.Contains(source, "@") {
		return source, nil
	}

	// Local path is accepted as-is.
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "~") {
		return source, nil
	}

	// org/repo: needs interactive disambiguation.
	// Keep option and interactive behavior consistent.
	if strings.Count(source, "/") == 1 {
		if !isTTY {
			return "", fmt.Errorf("ambiguous --source %q: provide a full URL, or run interactively to choose SSH/HTTPS", source)
		}
		orgRepo := strings.TrimSuffix(source, ".git")
		proto := promptGitHubProtocol(reader)
		return formatGitHubSource(proto, orgRepo), nil
	}

	// We intentionally do not support org-only here to keep behavior simple.
	// Use a full URL or org/repo.
	if strings.Count(source, "/") == 0 {
		return "", fmt.Errorf("invalid --source %q: expected URL, org/repo, or local path", source)
	}

	// Unknown; return as-is.
	return source, nil
}

func promptSourceInteractive(reader *bufio.Reader) string {
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

		// org/repo (or org/repo.git): ask SSH vs HTTPS.
		if strings.Count(v, "/") == 1 && !strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "~") {
			orgRepo := strings.TrimSuffix(v, ".git")
			proto := promptGitHubProtocol(reader)
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
	case "ssh":
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
