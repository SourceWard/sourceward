package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/SourceWard/sourceward/internal/inventory"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type editorAdapter struct {
	name    string
	command string
	runner  commandRunner
	files   fileSystem
}

func newEditorAdapter(name, command string, runner commandRunner) Adapter {
	return editorAdapter{
		name:    name,
		command: command,
		runner:  runner,
		files:   osFileSystem{},
	}
}

func (adapter editorAdapter) Name() string {
	return adapter.name
}

type extensionManifest struct {
	Publisher             string                     `json:"publisher"`
	Name                  string                     `json:"name"`
	Version               string                     `json:"version"`
	Main                  string                     `json:"main"`
	Browser               string                     `json:"browser"`
	ActivationEvents      []string                   `json:"activationEvents"`
	Contributes           map[string]json.RawMessage `json:"contributes"`
	ExtensionDependencies []string                   `json:"extensionDependencies"`
	ExtensionPack         []string                   `json:"extensionPack"`
	Scripts               map[string]string          `json:"scripts"`
	Dependencies          map[string]string          `json:"dependencies"`
}

func (adapter editorAdapter) Discover(ctx context.Context, options Options) Result {
	cliArtifacts, cliDiagnostics, cliAvailable := adapter.discoverCLI(ctx)
	packageArtifacts, packageDiagnostics, packagesInspected := adapter.discoverPackages(options)

	result := Result{
		Artifacts:   packageArtifacts,
		Diagnostics: append(cliDiagnostics, packageDiagnostics...),
	}
	packages := make(map[string][]inventory.Artifact, len(packageArtifacts))
	for _, artifact := range packageArtifacts {
		packages[artifact.ID] = append(packages[artifact.ID], artifact)
	}
	cli := make(map[string]inventory.Artifact, len(cliArtifacts))
	for _, artifact := range cliArtifacts {
		cli[artifact.ID] = artifact
		if installed, ok := packages[artifact.ID]; ok {
			versionMatched := false
			for _, candidate := range installed {
				if candidate.Version == artifact.Version {
					versionMatched = true
					break
				}
			}
			if !versionMatched {
				result.Diagnostics = append(result.Diagnostics, inventory.Diagnostic{
					Code:     "extension_inventory_mismatch",
					Level:    "warning",
					Provider: adapter.Name(),
					Message:  fmt.Sprintf("editor and package inventory report different versions for %q", artifact.Name),
					Path:     displayPath(installed[0].Path, options),
				})
			}
			continue
		}
		result.Artifacts = append(result.Artifacts, artifact)
		if !packagesInspected {
			continue
		}
		result.Diagnostics = append(result.Diagnostics, inventory.Diagnostic{
			Code:     "extension_inventory_mismatch",
			Level:    "warning",
			Provider: adapter.Name(),
			Message:  fmt.Sprintf("editor reports extension %q without an installed package", artifact.Name),
		})
	}
	reported := make(map[string]struct{})
	for _, artifact := range packageArtifacts {
		if _, ok := cli[artifact.ID]; ok || !cliAvailable {
			continue
		}
		if _, ok := reported[artifact.ID]; ok {
			continue
		}
		reported[artifact.ID] = struct{}{}
		result.Diagnostics = append(result.Diagnostics, inventory.Diagnostic{
			Code:     "extension_inventory_mismatch",
			Level:    "warning",
			Provider: adapter.Name(),
			Message:  fmt.Sprintf("installed package %q is absent from editor inventory", artifact.Name),
			Path:     displayPath(artifact.Path, options),
		})
	}
	return result
}

func (adapter editorAdapter) discoverCLI(ctx context.Context) ([]inventory.Artifact, []inventory.Diagnostic, bool) {
	output, err := adapter.runner(ctx, adapter.command, "--list-extensions", "--show-versions")
	if err != nil {
		level := "warning"
		code := "provider_failed"
		message := "editor extension inventory command failed"
		if isCommandUnavailable(err) {
			level = "info"
			code = "provider_unavailable"
			message = "editor command is not installed or not available on PATH"
		}
		return nil, []inventory.Diagnostic{{
			Code:     code,
			Level:    level,
			Provider: adapter.Name(),
			Message:  message,
		}}, false
	}

	var artifacts []inventory.Artifact
	var diagnostics []inventory.Diagnostic
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, version, ok := strings.Cut(line, "@")
		if !ok || name == "" || version == "" {
			diagnostics = append(diagnostics, inventory.Diagnostic{
				Code:     "malformed_output",
				Level:    "warning",
				Provider: adapter.Name(),
				Message:  "editor returned an extension without a name and version",
			})
			continue
		}
		publisher, _, _ := strings.Cut(name, ".")
		artifacts = append(artifacts, inventory.Artifact{
			ID:      adapter.name + ":" + name,
			Name:    name,
			Kind:    "ide-extension",
			Version: version,
			Source:  adapter.name,
			Scope:   "personal",
			Provenance: inventory.Provenance{
				Kind:      "marketplace",
				Provider:  adapter.name,
				Publisher: publisher,
				Package:   name,
			},
		})
	}
	return artifacts, diagnostics, true
}

func (adapter editorAdapter) discoverPackages(options Options) ([]inventory.Artifact, []inventory.Diagnostic, bool) {
	var artifacts []inventory.Artifact
	var diagnostics []inventory.Diagnostic
	inspected := false
	for _, root := range extensionPackageRoots(adapter.name, options.Home, runtime.GOOS) {
		found, foundDiagnostics, rootInspected := adapter.discoverPackageRoot(root, options)
		artifacts = append(artifacts, found...)
		diagnostics = append(diagnostics, foundDiagnostics...)
		inspected = inspected || rootInspected
	}
	return artifacts, diagnostics, inspected
}

func extensionPackageRoots(provider, home, goos string) []string {
	switch goos {
	case "darwin", "linux", "windows":
	default:
		return nil
	}
	switch provider {
	case "visual-studio-code":
		return []string{filepath.Join(home, ".vscode", "extensions")}
	case "cursor":
		return []string{filepath.Join(home, ".cursor", "extensions")}
	default:
		return nil
	}
}

func (adapter editorAdapter) discoverPackageRoot(
	root string,
	options Options,
) ([]inventory.Artifact, []inventory.Diagnostic, bool) {
	entries, err := adapter.files.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, false
	}
	if err != nil {
		return nil, []inventory.Diagnostic{{
			Code:     "location_unreadable",
			Level:    "error",
			Provider: adapter.Name(),
			Message:  "editor extension directory could not be read",
			Path:     displayPath(root, options),
		}}, false
	}

	var artifacts []inventory.Artifact
	var diagnostics []inventory.Diagnostic
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			diagnostics = append(diagnostics, adapter.packageDiagnostic(
				"artifact_unreadable",
				"symlinked editor extension package was not inspected",
				path,
				options,
			))
			continue
		}
		if !entry.IsDir() {
			continue
		}
		artifact, diagnostic := adapter.readPackage(path, options)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, diagnostics, true
}

func (adapter editorAdapter) readPackage(path string, options Options) (inventory.Artifact, *inventory.Diagnostic) {
	manifestPath := filepath.Join(path, "package.json")
	info, err := adapter.files.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() {
		diagnostic := adapter.packageDiagnostic(
			"artifact_unreadable",
			"editor extension manifest could not be inspected",
			manifestPath,
			options,
		)
		return inventory.Artifact{}, &diagnostic
	}
	content, err := adapter.files.ReadFile(manifestPath)
	if err != nil {
		diagnostic := adapter.packageDiagnostic(
			"artifact_unreadable",
			"editor extension manifest could not be read",
			manifestPath,
			options,
		)
		return inventory.Artifact{}, &diagnostic
	}
	var manifest extensionManifest
	if err := json.Unmarshal(content, &manifest); err != nil ||
		strings.TrimSpace(manifest.Publisher) == "" ||
		strings.TrimSpace(manifest.Name) == "" ||
		strings.TrimSpace(manifest.Version) == "" {
		diagnostic := adapter.packageDiagnostic(
			"malformed_artifact",
			"editor extension manifest is malformed",
			manifestPath,
			options,
		)
		return inventory.Artifact{}, &diagnostic
	}

	name := strings.TrimSpace(manifest.Publisher) + "." + strings.TrimSpace(manifest.Name)
	return inventory.Artifact{
		ID:        adapter.name + ":" + name,
		Name:      name,
		Kind:      "ide-extension",
		Version:   strings.TrimSpace(manifest.Version),
		Path:      displayPath(path, options),
		LocalPath: path,
		Source:    adapter.name,
		Scope:     "personal",
		Metadata:  extensionMetadata(manifest),
		Provenance: inventory.Provenance{
			Kind:      "marketplace",
			Provider:  adapter.name,
			Publisher: strings.TrimSpace(manifest.Publisher),
			Package:   name,
		},
	}, nil
}

func (adapter editorAdapter) packageDiagnostic(code, message, path string, options Options) inventory.Diagnostic {
	return inventory.Diagnostic{
		Code:     code,
		Level:    "warning",
		Provider: adapter.Name(),
		Message:  message,
		Path:     displayPath(path, options),
	}
}

func extensionMetadata(manifest extensionManifest) map[string]string {
	capabilities := make([]string, 0, len(manifest.Contributes)+5)
	for capability := range manifest.Contributes {
		capabilities = append(capabilities, "contributes."+capability)
	}
	if manifest.Main != "" {
		capabilities = append(capabilities, "node-runtime")
	}
	if manifest.Browser != "" {
		capabilities = append(capabilities, "web-runtime")
	}
	if len(manifest.ActivationEvents) != 0 {
		capabilities = append(capabilities, "activation-events")
	}
	if len(manifest.ExtensionDependencies) != 0 {
		capabilities = append(capabilities, "extension-dependencies")
	}
	if len(manifest.ExtensionPack) != 0 {
		capabilities = append(capabilities, "extension-pack")
	}
	sort.Strings(capabilities)

	metadata := map[string]string{
		"publisher": manifest.Publisher,
	}
	if len(capabilities) != 0 {
		metadata["capabilities"] = strings.Join(capabilities, ",")
	}
	if containsString(manifest.ActivationEvents, "*") {
		metadata["risk_broad_activation"] = "true"
	}
	if scripts := selectedKeys(manifest.Scripts, []string{"preinstall", "install", "postinstall"}); len(scripts) != 0 {
		metadata["risk_install_scripts"] = strings.Join(scripts, ",")
	}
	if dependencies := selectedKeys(manifest.Dependencies, []string{
		"cross-spawn", "execa", "shelljs", "sudo-prompt",
	}); len(dependencies) != 0 {
		metadata["risk_process_dependencies"] = strings.Join(dependencies, ",")
	}
	if dependencies := selectedKeys(manifest.Dependencies, []string{
		"axios", "got", "node-fetch", "request", "undici", "ws",
	}); len(dependencies) != 0 {
		metadata["risk_network_dependencies"] = strings.Join(dependencies, ",")
	}
	if dependencies := selectedKeys(manifest.Dependencies, []string{
		"javascript-obfuscator", "js-confuser", "obfuscator-io",
	}); len(dependencies) != 0 {
		metadata["risk_obfuscation_dependencies"] = strings.Join(dependencies, ",")
	}
	return metadata
}

func selectedKeys(values map[string]string, selected []string) []string {
	var result []string
	for _, key := range selected {
		if _, ok := values[key]; ok {
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result
}

func isCommandUnavailable(err error) bool {
	var execError *exec.Error
	return errors.As(err, &execError) && errors.Is(execError.Err, exec.ErrNotFound)
}

func execCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
