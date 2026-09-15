package discovery

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestEditorAdapterDiscoversInstalledPackageWithoutExecutingIt(t *testing.T) {
	home := t.TempDir()
	packagePath := filepath.Join(home, ".vscode", "extensions", "acme.tool-1.2.3")
	writeTestFile(t, filepath.Join(packagePath, "package.json"), `{
  "publisher": "acme",
  "name": "tool",
  "version": "1.2.3",
  "main": "./extension.js",
  "browser": "./browser.js",
  "activationEvents": ["onStartupFinished"],
  "extensionDependencies": ["other.dependency"],
  "extensionPack": ["other.pack"],
  "contributes": {
    "commands": [{"command": "acme.tool.run", "title": "Run"}],
    "configuration": {}
  }
}`)
	writeTestFile(t, filepath.Join(packagePath, "extension.js"), `throw new Error("must not execute");`)

	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, &exec.Error{Name: "code", Err: exec.ErrNotFound}
	}
	result := newEditorAdapter("visual-studio-code", "code", runner).
		Discover(context.Background(), Options{Root: t.TempDir(), Home: home})

	if len(result.Artifacts) != 1 {
		t.Fatalf("got %d artifacts, want 1: %#v", len(result.Artifacts), result.Artifacts)
	}
	artifact := result.Artifacts[0]
	if artifact.ID != "visual-studio-code:acme.tool" ||
		artifact.Name != "acme.tool" ||
		artifact.Version != "1.2.3" ||
		artifact.Path != packagePath {
		t.Fatalf("unexpected artifact %#v", artifact)
	}
	if artifact.Metadata["publisher"] != "acme" {
		t.Fatalf("unexpected publisher metadata %#v", artifact.Metadata)
	}
	if artifact.Provenance.Kind != "marketplace" ||
		artifact.Provenance.Provider != "visual-studio-code" ||
		artifact.Provenance.Publisher != "acme" ||
		artifact.Provenance.Package != "acme.tool" {
		t.Fatalf("unexpected provenance %#v", artifact.Provenance)
	}
	wantCapabilities := "activation-events,contributes.commands,contributes.configuration,extension-dependencies,extension-pack,node-runtime,web-runtime"
	if artifact.Metadata["capabilities"] != wantCapabilities {
		t.Fatalf("got capabilities %q, want %q", artifact.Metadata["capabilities"], wantCapabilities)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "provider_unavailable" {
		t.Fatalf("unexpected diagnostics %#v", result.Diagnostics)
	}
}

func TestEditorAdapterReportsCLIAndPackageMismatch(t *testing.T) {
	home := t.TempDir()
	packagePath := filepath.Join(home, ".cursor", "extensions", "acme.tool-1.0.0")
	writeTestFile(t, filepath.Join(packagePath, "package.json"), `{
  "publisher": "acme",
  "name": "tool",
  "version": "1.0.0"
}`)

	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("acme.tool@2.0.0\ncli.only@3.0.0\n"), nil
	}

	result := newEditorAdapter("cursor", "cursor", runner).
		Discover(context.Background(), Options{Root: t.TempDir(), Home: home})

	if len(result.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2: %#v", len(result.Artifacts), result.Artifacts)
	}

	if got := countDiagnostics(result, "extension_inventory_mismatch"); got != 2 {
		t.Fatalf("got %d mismatch diagnostics, want 2: %#v", got, result.Diagnostics)
	}
}

func TestExtensionManifestDerivesRiskSignals(t *testing.T) {
	metadata := extensionMetadata(extensionManifest{
		ActivationEvents: []string{"*"},
		Scripts: map[string]string{
			"postinstall": "node setup.js",
			"test":        "go test",
		},
		Dependencies: map[string]string{
			"execa":                 "1.0.0",
			"undici":                "1.0.0",
			"javascript-obfuscator": "1.0.0",
			"safe-package":          "1.0.0",
		},
	})
	want := map[string]string{
		"risk_broad_activation":         "true",
		"risk_install_scripts":          "postinstall",
		"risk_process_dependencies":     "execa",
		"risk_network_dependencies":     "undici",
		"risk_obfuscation_dependencies": "javascript-obfuscator",
	}
	for key, value := range want {
		if metadata[key] != value {
			t.Fatalf("metadata[%q] = %q, want %q: %#v", key, metadata[key], value, metadata)
		}
	}
	safe := extensionMetadata(extensionManifest{
		ActivationEvents: []string{"onCommand:example.safe"},
		Scripts:          map[string]string{"test": "go test"},
		Dependencies:     map[string]string{"safe-package": "1.0.0"},
	})
	for key := range want {
		if safe[key] != "" {
			t.Fatalf("unexpected safe signal %q in %#v", key, safe)
		}
	}
}

func TestEditorAdapterMatchesCLIToAnyInstalledPackageVersion(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"1.0.0", "2.0.0"} {
		packagePath := filepath.Join(home, ".vscode", "extensions", "acme.tool-"+version)
		writeTestFile(t, filepath.Join(packagePath, "package.json"), `{
  "publisher": "acme",
  "name": "tool",
  "version": "`+version+`"
}`)
	}

	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("acme.tool@2.0.0\n"), nil
	}
	result := newEditorAdapter("visual-studio-code", "code", runner).
		Discover(context.Background(), Options{Root: t.TempDir(), Home: home})

	if len(result.Artifacts) != 2 {
		t.Fatalf("got %d package artifacts, want 2: %#v", len(result.Artifacts), result.Artifacts)
	}
	if got := countDiagnostics(result, "extension_inventory_mismatch"); got != 0 {
		t.Fatalf("got %d mismatch diagnostics, want 0: %#v", got, result.Diagnostics)
	}
}

func TestEditorAdapterRejectsSymlinkedPackageAndManifest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated Windows privileges")
	}
	home := t.TempDir()
	root := filepath.Join(home, ".vscode", "extensions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	externalPackage := filepath.Join(t.TempDir(), "external-package")
	writeTestFile(t, filepath.Join(externalPackage, "package.json"), `{
  "publisher": "external",
  "name": "package",
  "version": "1.0.0"
}`)
	if err := os.Symlink(externalPackage, filepath.Join(root, "external.package-1.0.0")); err != nil {
		t.Fatal(err)
	}

	manifestTarget := filepath.Join(t.TempDir(), "package.json")
	writeTestFile(t, manifestTarget, `{
  "publisher": "external",
  "name": "manifest",
  "version": "1.0.0"
}`)
	manifestPackage := filepath.Join(root, "external.manifest-1.0.0")
	if err := os.MkdirAll(manifestPackage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(manifestTarget, filepath.Join(manifestPackage, "package.json")); err != nil {
		t.Fatal(err)
	}

	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, &exec.Error{Name: "code", Err: exec.ErrNotFound}
	}
	result := newEditorAdapter("visual-studio-code", "code", runner).
		Discover(context.Background(), Options{Root: t.TempDir(), Home: home})

	if len(result.Artifacts) != 0 {
		t.Fatalf("symlinked packages were inventoried: %#v", result.Artifacts)
	}
	if got := countDiagnostics(result, "artifact_unreadable"); got != 2 {
		t.Fatalf("got %d unreadable diagnostics, want 2: %#v", got, result.Diagnostics)
	}
}

func TestExtensionPackageRootsSupportDesktopPlatforms(t *testing.T) {
	home := filepath.Join("home", "person")
	for _, goos := range []string{"darwin", "linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			vscode := extensionPackageRoots("visual-studio-code", home, goos)
			cursor := extensionPackageRoots("cursor", home, goos)
			if !reflect.DeepEqual(vscode, []string{filepath.Join(home, ".vscode", "extensions")}) {
				t.Fatalf("unexpected VS Code roots %#v", vscode)
			}
			if !reflect.DeepEqual(cursor, []string{filepath.Join(home, ".cursor", "extensions")}) {
				t.Fatalf("unexpected Cursor roots %#v", cursor)
			}
		})
	}
	if roots := extensionPackageRoots("visual-studio-code", home, "plan9"); roots != nil {
		t.Fatalf("unexpected unsupported platform roots %#v", roots)
	}
}

func TestEditorAdapterReportsUnreadablePackageRoot(t *testing.T) {
	adapter := editorAdapter{
		name:    "visual-studio-code",
		command: "code",
		runner: func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("failed")
		},
		files: failingFileSystem{readDirError: os.ErrPermission},
	}
	result := adapter.Discover(context.Background(), Options{Root: "/repo", Home: "/home/user"})
	if countDiagnostics(result, "location_unreadable") != 1 {
		t.Fatalf("unexpected diagnostics %#v", result.Diagnostics)
	}
}

func countDiagnostics(result Result, code string) int {
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == code {
			count++
		}
	}
	return count
}
