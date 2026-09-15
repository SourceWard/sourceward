package lockfile

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/SourceWard/sourceward/internal/inventory"
)

const SchemaVersion = 2

var ErrDrift = errors.New("lockfile does not match discovered artifacts")

type Lockfile struct {
	SchemaVersion int        `json:"schema_version"`
	Artifacts     []Artifact `json:"artifacts"`
}

type Artifact struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Kind       string               `json:"kind"`
	Version    string               `json:"version,omitempty"`
	Scope      string               `json:"scope"`
	Source     string               `json:"source"`
	Provenance inventory.Provenance `json:"provenance"`
	Integrity  Integrity            `json:"integrity"`
}

type Integrity struct {
	Scope     string `json:"scope"`
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
}

type Options struct {
	Root            string
	IncludePersonal bool
}

func Generate(found inventory.Inventory, options Options) (Lockfile, error) {
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return Lockfile{}, fmt.Errorf("resolve repository root: %w", err)
	}

	locked := Lockfile{
		SchemaVersion: SchemaVersion,
		Artifacts:     make([]Artifact, 0, len(found.Artifacts)),
	}
	for _, artifact := range found.Artifacts {
		if artifact.Scope != "project" && !options.IncludePersonal {
			continue
		}

		lockedArtifact := Artifact{
			ID:         artifact.ID,
			Name:       artifact.Name,
			Kind:       artifact.Kind,
			Version:    artifact.Version,
			Scope:      artifact.Scope,
			Source:     normalizeSource(artifact.Source, artifact.Scope, root),
			Provenance: artifact.Provenance,
		}
		if lockedArtifact.Provenance.Kind == "" {
			lockedArtifact.Provenance.Kind = "unknown"
		}
		integrity, err := artifactIntegrity(artifact, lockedArtifact)
		if err != nil {
			return Lockfile{}, fmt.Errorf("hash %s: %w", artifact.ID, err)
		}
		lockedArtifact.Integrity = integrity
		locked.Artifacts = append(locked.Artifacts, lockedArtifact)
	}

	sort.Slice(locked.Artifacts, func(i, j int) bool {
		left := locked.Artifacts[i]
		right := locked.Artifacts[j]
		if left.ID != right.ID {
			return left.ID < right.ID
		}
		if left.Scope != right.Scope {
			return left.Scope < right.Scope
		}
		return left.Source < right.Source
	})
	return locked, nil
}

func Write(path string, locked Lockfile) error {
	content, err := marshal(locked)
	if err != nil {
		return err
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create lockfile directory: %w", err)
	}
	temp, err := os.CreateTemp(directory, ".sourceward-lock-*")
	if err != nil {
		return fmt.Errorf("create temporary lockfile: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return fmt.Errorf("set lockfile permissions: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary lockfile: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary lockfile: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary lockfile: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace lockfile: %w", err)
	}
	return nil
}

func Check(path string, current Lockfile) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read lockfile: %w", err)
	}

	var existing Lockfile
	if err := json.Unmarshal(content, &existing); err != nil {
		return fmt.Errorf("decode lockfile: %w", err)
	}
	if existing.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported lockfile schema version %d", existing.SchemaVersion)
	}
	if !reflect.DeepEqual(existing, current) {
		return ErrDrift
	}
	return nil
}

func marshal(locked Lockfile) ([]byte, error) {
	content, err := json.MarshalIndent(locked, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode lockfile: %w", err)
	}
	return append(content, '\n'), nil
}

func artifactIntegrity(artifact inventory.Artifact, locked Artifact) (Integrity, error) {
	if contentBacked(artifact) {
		path := artifact.Path
		if artifact.Kind == "agent-skill" {
			path = filepath.Dir(path)
		}
		digest, err := hashPath(path)
		if err != nil {
			return Integrity{}, err
		}
		return Integrity{Scope: "content", Algorithm: "sha256", Digest: digest}, nil
	}

	content, err := json.Marshal(struct {
		ID         string               `json:"id"`
		Name       string               `json:"name"`
		Kind       string               `json:"kind"`
		Version    string               `json:"version"`
		Source     string               `json:"source"`
		Scope      string               `json:"scope"`
		Metadata   map[string]string    `json:"metadata,omitempty"`
		Provenance inventory.Provenance `json:"provenance"`
	}{
		ID:         locked.ID,
		Name:       locked.Name,
		Kind:       locked.Kind,
		Version:    locked.Version,
		Source:     locked.Source,
		Scope:      locked.Scope,
		Metadata:   artifact.Metadata,
		Provenance: locked.Provenance,
	})
	if err != nil {
		return Integrity{}, fmt.Errorf("encode artifact metadata: %w", err)
	}
	sum := sha256.Sum256(content)
	return Integrity{
		Scope:     "metadata",
		Algorithm: "sha256",
		Digest:    hex.EncodeToString(sum[:]),
	}, nil
}

func contentBacked(artifact inventory.Artifact) bool {
	return artifact.Path != "" &&
		(artifact.Kind == "agent-skill" || artifact.Kind == "ide-extension")
}

func hashPath(root string) (string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return hashFile(root, filepath.Base(root), info)
	}

	hash := sha256.New()
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return writeFileRecord(hash, path, filepath.ToSlash(relative), info)
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashFile(path, name string, info fs.FileInfo) (string, error) {
	hash := sha256.New()
	if err := writeFileRecord(hash, path, filepath.ToSlash(name), info); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeFileRecord(writer io.Writer, path, name string, info fs.FileInfo) error {
	var kind byte
	var content []byte
	var err error

	switch {
	case info.Mode().IsRegular():
		kind = 'f'
		content, err = os.ReadFile(path)
	case info.Mode()&os.ModeSymlink != 0:
		kind = 'l'
		var target string
		target, err = os.Readlink(path)
		content = []byte(target)
	default:
		return fmt.Errorf("unsupported file type %s", path)
	}
	if err != nil {
		return err
	}

	if _, err := writer.Write([]byte{kind}); err != nil {
		return err
	}
	if err := writeBytes(writer, []byte(name)); err != nil {
		return err
	}
	executable := byte(0)
	if info.Mode().Perm()&0o111 != 0 {
		executable = 1
	}
	if _, err := writer.Write([]byte{executable}); err != nil {
		return err
	}
	return writeBytes(writer, content)
}

func writeBytes(writer io.Writer, value []byte) error {
	if err := binary.Write(writer, binary.BigEndian, uint64(len(value))); err != nil {
		return err
	}
	_, err := writer.Write(value)
	return err
}

func normalizeSource(source, scope, root string) string {
	if scope != "project" {
		return source
	}
	absolute, err := filepath.Abs(source)
	if err != nil {
		return source
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return source
	}
	return filepath.ToSlash(relative)
}
