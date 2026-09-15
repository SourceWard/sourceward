package discovery

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/SourceWard/sourceward/internal/inventory"
)

func enrichProvenance(
	ctx context.Context,
	found inventory.Inventory,
	_ Options,
	runner commandRunner,
) inventory.Inventory {
	for index := range found.Artifacts {
		artifact := &found.Artifacts[index]
		if artifact.Provenance.Kind != "" {
			continue
		}
		if artifact.Kind == "agent-skill" {
			artifact.Provenance = gitProvenance(ctx, *artifact, runner)
			continue
		}
		artifact.Provenance = inventory.Provenance{Kind: "unknown"}
	}
	return found
}

func gitProvenance(
	ctx context.Context,
	artifact inventory.Artifact,
	runner commandRunner,
) inventory.Provenance {
	path := artifact.Path
	if artifact.Kind == "agent-skill" {
		path = filepath.Dir(path)
	}
	rootOutput, err := runner(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
	if err != nil {
		return inventory.Provenance{Kind: "unknown"}
	}
	root := strings.TrimSpace(string(rootOutput))
	revisionOutput, err := runner(ctx, "git", "-C", root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return inventory.Provenance{Kind: "unknown"}
	}

	provenance := inventory.Provenance{
		Kind:     "git",
		Revision: strings.TrimSpace(string(revisionOutput)),
	}
	root = canonicalPath(root)
	path = canonicalPath(path)
	if relative, err := filepath.Rel(root, path); err == nil &&
		relative != "." &&
		relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		provenance.Subdir = filepath.ToSlash(relative)
	}
	if remoteOutput, err := runner(ctx, "git", "-C", root, "config", "--get", "remote.origin.url"); err == nil {
		provenance.Repository = sanitizeRepositoryURL(strings.TrimSpace(string(remoteOutput)))
	}
	pathspec := provenance.Subdir
	if pathspec == "" {
		pathspec = "."
	}
	if statusOutput, err := runner(ctx, "git", "-C", root, "status", "--porcelain", "--", pathspec); err == nil {
		dirty := len(strings.TrimSpace(string(statusOutput))) != 0
		provenance.Dirty = &dirty
	}
	return provenance
}

func canonicalPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

func sanitizeRepositoryURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		if at := strings.LastIndex(value, "@"); at >= 0 {
			if colon := strings.Index(value[at+1:], ":"); colon >= 0 {
				hostStart := at + 1
				hostEnd := hostStart + colon
				host := value[hostStart:hostEnd]
				path := strings.TrimPrefix(value[hostEnd+1:], "/")
				if host != "" && path != "" {
					return "ssh://" + host + "/" + path
				}
			}
		}
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "ssh", "git":
	default:
		return ""
	}
	if parsed.Hostname() == "" {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
