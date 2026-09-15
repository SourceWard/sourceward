package discovery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SourceWard/sourceward/internal/inventory"
)

func TestMCPAdapterDiscoversJSONProvidersWithoutSecretValues(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".vscode", "mcp.json"), `{
  "servers": {
    "local": {
      "command": "/usr/local/bin/npx",
      "args": ["-y", "@example/server", "--token", "argument-secret"],
      "env": {
        "API_TOKEN": "environment-secret"
      }
    },
    "remote": {
      "type": "http",
      "url": "https://user:password@mcp.example.com/path?token=query-secret",
      "headers": {
        "Authorization": "Bearer header-secret",
        "X-Tenant": "tenant-secret"
      }
    }
  }
}`)
	writeTestFile(t, filepath.Join(root, ".cursor", "mcp.json"), `{
  "mcpServers": {
    "cursor-server": {
      "url": "https://cursor.example.com/mcp"
    }
  }
}`)
	writeTestFile(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "shared": {
      "command": "node"
    }
  }
}`)

	result := newMCPAdapter().Discover(context.Background(), Options{Root: root, Home: home})
	if len(result.Artifacts) != 4 {
		t.Fatalf("got %d artifacts, want 4: %#v", len(result.Artifacts), result.Artifacts)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	output := string(encoded)
	for _, secret := range []string{
		"argument-secret",
		"environment-secret",
		"password",
		"query-secret",
		"header-secret",
		"tenant-secret",
	} {
		if strings.Contains(output, secret) {
			t.Fatalf("secret %q leaked in %s", secret, output)
		}
	}

	local := findArtifact(t, result, "mcp:visual-studio-code:project:local")
	if local.Provenance.Kind != "configuration" ||
		local.Provenance.Provider != "visual-studio-code" ||
		local.Provenance.Package != "local" {
		t.Fatalf("unexpected local provenance %#v", local.Provenance)
	}
	if local.Metadata["transport"] != "stdio" || local.Metadata["command"] != "npx" {
		t.Fatalf("unexpected local metadata %#v", local.Metadata)
	}
	if local.Metadata["environment_variables"] != "API_TOKEN" {
		t.Fatalf("unexpected environment names %#v", local.Metadata)
	}
	remote := findArtifact(t, result, "mcp:visual-studio-code:project:remote")
	if remote.Metadata["endpoint_host"] != "mcp.example.com" {
		t.Fatalf("unexpected remote metadata %#v", remote.Metadata)
	}
	if remote.Metadata["endpoint_scheme"] != "https" {
		t.Fatalf("unexpected remote scheme %#v", remote.Metadata)
	}
	if remote.Metadata["header_names"] != "Authorization,X-Tenant" {
		t.Fatalf("unexpected header names %#v", remote.Metadata)
	}
}

func TestMCPAdapterDiscoversCodexTOMLWithoutSecretValues(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), `
[mcp_servers.local]
command = "/opt/tools/server"
args = ["--token", "argument-secret"]
enabled = false
env = { API_TOKEN = "environment-secret" }
env_vars = ["SHARED_TOKEN", { name = "REMOTE_TOKEN", source = "remote" }]

[mcp_servers.remote]
url = "https://user:password@api.example.com/mcp?token=query-secret"
bearer_token_env_var = "BEARER_TOKEN"
http_headers = { Authorization = "header-secret" }
env_http_headers = { "X-API-Key" = "API_KEY_ENV" }
`)

	result := newMCPAdapter().Discover(context.Background(), Options{Root: root, Home: home})
	if len(result.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2: %#v", len(result.Artifacts), result.Artifacts)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	output := string(encoded)
	for _, secret := range []string{
		"argument-secret",
		"environment-secret",
		"password",
		"query-secret",
		"header-secret",
		"API_KEY_ENV",
	} {
		if strings.Contains(output, secret) {
			t.Fatalf("secret %q leaked in %s", secret, output)
		}
	}

	local := findArtifact(t, result, "mcp:codex:personal:local")
	if local.Metadata["command"] != "server" || local.Metadata["enabled"] != "false" {
		t.Fatalf("unexpected local metadata %#v", local.Metadata)
	}
	if local.Metadata["environment_variables"] != "API_TOKEN,REMOTE_TOKEN,SHARED_TOKEN" {
		t.Fatalf("unexpected environment names %#v", local.Metadata)
	}
	remote := findArtifact(t, result, "mcp:codex:personal:remote")
	if remote.Metadata["endpoint_host"] != "api.example.com" {
		t.Fatalf("unexpected remote metadata %#v", remote.Metadata)
	}
	if remote.Metadata["environment_variables"] != "BEARER_TOKEN" {
		t.Fatalf("unexpected bearer metadata %#v", remote.Metadata)
	}
	if remote.Metadata["header_names"] != "Authorization,X-API-Key" {
		t.Fatalf("unexpected header metadata %#v", remote.Metadata)
	}
}

func TestMCPRiskSignalsAreDerivedWithoutValues(t *testing.T) {
	signals := mcpRiskSignals(
		"bash",
		[]string{"-c", "npx package-name", "/"},
		"http://remote.example.invalid/mcp?token=value",
		map[string]json.RawMessage{"API_TOKEN": json.RawMessage(`"literal-value"`)},
		map[string]json.RawMessage{"Authorization": json.RawMessage(`"literal-value"`)},
	)
	for _, key := range []string{
		"risk_shell_execution",
		"risk_inline_credentials",
		"risk_broad_filesystem",
		"risk_insecure_transport",
		"risk_sensitive_environment",
	} {
		if signals[key] == "" {
			t.Fatalf("missing signal %q in %#v", key, signals)
		}
	}
	encoded, err := json.Marshal(signals)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "literal-value") {
		t.Fatalf("credential value leaked in %#v", signals)
	}

	unpinned := mcpRiskSignals("npx", []string{"package-name"}, "", nil, nil)
	if unpinned["risk_unpinned_package"] != "true" {
		t.Fatalf("missing unpinned signal %#v", unpinned)
	}
	safe := mcpRiskSignals(
		"npx",
		[]string{"package-name@1.2.3"},
		"http://localhost:3000/mcp",
		map[string]json.RawMessage{"API_TOKEN": json.RawMessage(`"${API_TOKEN}"`)},
		nil,
	)
	if safe["risk_unpinned_package"] != "" ||
		safe["risk_insecure_transport"] != "" ||
		safe["risk_inline_credentials"] != "" {
		t.Fatalf("unexpected safe signals %#v", safe)
	}
}

func TestMCPAdapterDiscoversClaudeLocalScopeForCurrentProject(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	content := `{
  "mcpServers": {
    "user-server": {"command": "node"}
  },
  "projects": {
    ` + mustJSON(t, root) + `: {
      "mcpServers": {
        "local-server": {"url": "https://local.example.com/mcp"}
      }
    },
    "/another/project": {
      "mcpServers": {
        "other-server": {"command": "other"}
      }
    }
  }
}`
	writeTestFile(t, filepath.Join(home, ".claude.json"), content)

	result := newMCPAdapter().Discover(context.Background(), Options{Root: root, Home: home})
	if len(result.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2: %#v", len(result.Artifacts), result.Artifacts)
	}
	findArtifact(t, result, "mcp:claude-code:personal:user-server")
	local := findArtifact(t, result, "mcp:claude-code:local:local-server")
	if local.Metadata["endpoint_host"] != "local.example.com" {
		t.Fatalf("unexpected local metadata %#v", local.Metadata)
	}
}

func TestMCPAdapterDistinguishesProviderAndScope(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	config := `{"mcpServers":{"shared":{"command":"node"}}}`
	writeTestFile(t, filepath.Join(root, ".cursor", "mcp.json"), config)
	writeTestFile(t, filepath.Join(home, ".cursor", "mcp.json"), config)
	writeTestFile(t, filepath.Join(root, ".mcp.json"), config)

	result := newMCPAdapter().Discover(context.Background(), Options{Root: root, Home: home})
	ids := map[string]bool{}
	for _, artifact := range result.Artifacts {
		ids[artifact.ID] = true
	}
	for _, expected := range []string{
		"mcp:cursor:project:shared",
		"mcp:cursor:personal:shared",
		"mcp:portable-mcp:project:shared",
	} {
		if !ids[expected] {
			t.Fatalf("missing %s in %#v", expected, ids)
		}
	}
}

func TestMCPAdapterReportsMalformedAndUnsupportedConfigurations(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".vscode", "mcp.json"), `{broken`)
	writeTestFile(t, filepath.Join(root, ".cursor", "mcp.json"), `{
  "mcpServers": {
    "missing-transport": {},
    "ambiguous": {
      "command": "node",
      "url": "https://example.com/mcp"
    },
    "invalid-url": {
      "type": "http",
      "url": "not-a-url"
    },
    "unknown-type": {
      "type": "websocket",
      "url": "https://example.com/mcp"
    }
  }
}`)

	result := newMCPAdapter().Discover(context.Background(), Options{Root: root, Home: home})
	codes := map[string]int{}
	for _, diagnostic := range result.Diagnostics {
		codes[diagnostic.Code]++
		if strings.Contains(diagnostic.Message, "{broken") {
			t.Fatal("raw configuration leaked into diagnostic")
		}
	}
	if codes["malformed_configuration"] != 1 || codes["unsupported_transport"] != 4 {
		t.Fatalf("unexpected diagnostics %#v", result.Diagnostics)
	}
	if len(result.Artifacts) != 4 {
		t.Fatalf("unsupported servers should remain inventoried: %#v", result.Artifacts)
	}
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func findArtifact(t *testing.T, result Result, id string) inventory.Artifact {
	t.Helper()
	for _, artifact := range result.Artifacts {
		if artifact.ID == id {
			return artifact
		}
	}
	t.Fatalf("artifact %s not found in %#v", id, result.Artifacts)
	return inventory.Artifact{}
}
