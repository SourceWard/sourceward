package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/SourceWard/sourceward/internal/inventory"
)

type mcpAdapter struct {
	files fileSystem
}

type mcpLocation struct {
	path      string
	provider  string
	scope     string
	format    string
	consumers []string
}

type jsonMCPConfig struct {
	Servers    map[string]json.RawMessage `json:"servers"`
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

type claudeConfig struct {
	Projects map[string]struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	} `json:"projects"`
}

type jsonMCPServer struct {
	Type     string                     `json:"type"`
	Command  string                     `json:"command"`
	Args     []string                   `json:"args"`
	URL      string                     `json:"url"`
	Env      map[string]json.RawMessage `json:"env"`
	Headers  map[string]json.RawMessage `json:"headers"`
	Disabled bool                       `json:"disabled"`
}

type codexConfig struct {
	MCPServers map[string]codexMCPServer `toml:"mcp_servers"`
}

type codexMCPServer struct {
	Command           string            `toml:"command"`
	Args              []string          `toml:"args"`
	URL               string            `toml:"url"`
	Env               map[string]any    `toml:"env"`
	EnvVars           []any             `toml:"env_vars"`
	BearerTokenEnvVar string            `toml:"bearer_token_env_var"`
	HTTPHeaders       map[string]string `toml:"http_headers"`
	EnvHTTPHeaders    map[string]string `toml:"env_http_headers"`
	Enabled           *bool             `toml:"enabled"`
}

type normalizedMCPServer struct {
	name           string
	transport      string
	command        string
	endpointScheme string
	endpointHost   string
	envNames       []string
	headerNames    []string
	enabled        bool
	riskSignals    map[string]string
}

func newMCPAdapter() Adapter {
	return mcpAdapter{files: osFileSystem{}}
}

func (mcpAdapter) Name() string {
	return "mcp"
}

func (adapter mcpAdapter) Discover(_ context.Context, options Options) Result {
	locations := []mcpLocation{
		{
			path:      filepath.Join(options.Root, ".mcp.json"),
			provider:  "portable-mcp",
			scope:     "project",
			format:    "json",
			consumers: []string{"claude-code", "github-copilot", "visual-studio-code"},
		},
		{
			path:     filepath.Join(options.Root, ".vscode", "mcp.json"),
			provider: "visual-studio-code",
			scope:    "project",
			format:   "json",
		},
		{
			path:     filepath.Join(options.Root, ".cursor", "mcp.json"),
			provider: "cursor",
			scope:    "project",
			format:   "json",
		},
		{
			path:     filepath.Join(options.Root, ".codex", "config.toml"),
			provider: "codex",
			scope:    "project",
			format:   "toml",
		},
		{
			path:     filepath.Join(options.Home, ".copilot", "mcp-config.json"),
			provider: "github-copilot",
			scope:    "personal",
			format:   "json",
		},
		{
			path:     filepath.Join(options.Home, ".cursor", "mcp.json"),
			provider: "cursor",
			scope:    "personal",
			format:   "json",
		},
		{
			path:     filepath.Join(options.Home, ".claude.json"),
			provider: "claude-code",
			scope:    "personal",
			format:   "json",
		},
		{
			path:     filepath.Join(options.Home, ".claude.json"),
			provider: "claude-code",
			scope:    "local",
			format:   "claude-local-json",
		},
		{
			path:     filepath.Join(options.Home, ".codex", "config.toml"),
			provider: "codex",
			scope:    "personal",
			format:   "toml",
		},
	}

	result := Result{}
	for _, location := range locations {
		found := adapter.discoverLocation(location, options)
		result.Artifacts = append(result.Artifacts, found.Artifacts...)
		result.Diagnostics = append(result.Diagnostics, found.Diagnostics...)
	}
	return result
}

func (adapter mcpAdapter) discoverLocation(location mcpLocation, options Options) Result {
	content, err := adapter.files.ReadFile(location.path)
	if os.IsNotExist(err) {
		return Result{}
	}
	if err != nil {
		return Result{Diagnostics: []inventory.Diagnostic{{
			Code:     "location_unreadable",
			Level:    "error",
			Provider: location.provider,
			Message:  "MCP configuration could not be read",
			Path:     displayPath(location.path, options),
		}}}
	}

	var servers []normalizedMCPServer
	switch location.format {
	case "json":
		servers, err = parseJSONMCP(content)
	case "toml":
		servers, err = parseCodexMCP(content)
	case "claude-local-json":
		servers, err = parseClaudeLocalMCP(content, options.Root)
	default:
		err = fmt.Errorf("unsupported configuration format")
	}
	if err != nil {
		return Result{Diagnostics: []inventory.Diagnostic{{
			Code:     "malformed_configuration",
			Level:    "warning",
			Provider: location.provider,
			Message:  "MCP configuration could not be parsed",
			Path:     displayPath(location.path, options),
		}}}
	}

	result := Result{}
	for _, server := range servers {
		artifact, diagnostic := mcpArtifact(location, server, options)
		result.Artifacts = append(result.Artifacts, artifact)
		if diagnostic != nil {
			result.Diagnostics = append(result.Diagnostics, *diagnostic)
		}
	}
	return result
}

func parseClaudeLocalMCP(content []byte, root string) ([]normalizedMCPServer, error) {
	var config claudeConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, err
	}
	for projectPath, project := range config.Projects {
		if samePath(projectPath, root) {
			return parseJSONMCPServers(project.MCPServers)
		}
	}
	return []normalizedMCPServer{}, nil
}

func parseJSONMCP(content []byte) ([]normalizedMCPServer, error) {
	var config jsonMCPConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, err
	}
	servers := config.MCPServers
	if servers == nil {
		servers = config.Servers
	}
	return parseJSONMCPServers(servers)
}

func parseJSONMCPServers(servers map[string]json.RawMessage) ([]normalizedMCPServer, error) {
	names := sortedKeys(servers)
	result := make([]normalizedMCPServer, 0, len(names))
	for _, name := range names {
		var server jsonMCPServer
		if err := json.Unmarshal(servers[name], &server); err != nil {
			return nil, err
		}
		result = append(result, normalizeMCPServer(
			name,
			server.Type,
			server.Command,
			server.Args,
			server.URL,
			server.Env,
			server.Headers,
			!server.Disabled,
		))
	}
	return result, nil
}

func parseCodexMCP(content []byte) ([]normalizedMCPServer, error) {
	var config codexConfig
	if err := toml.Unmarshal(content, &config); err != nil {
		return nil, err
	}
	names := sortedKeys(config.MCPServers)
	result := make([]normalizedMCPServer, 0, len(names))
	for _, name := range names {
		server := config.MCPServers[name]
		envNames := mapKeys(server.Env)
		envNames = append(envNames, codexEnvVarNames(server.EnvVars)...)
		if server.BearerTokenEnvVar != "" {
			envNames = append(envNames, server.BearerTokenEnvVar)
		}
		headerNames := mapKeys(server.HTTPHeaders)
		headerNames = append(headerNames, mapKeys(server.EnvHTTPHeaders)...)
		enabled := true
		if server.Enabled != nil {
			enabled = *server.Enabled
		}
		result = append(result, normalizeMCPServer(
			name,
			"",
			server.Command,
			server.Args,
			server.URL,
			mapAsRaw(server.Env),
			mapAsRaw(server.HTTPHeaders),
			enabled,
		))
		result[len(result)-1].envNames = uniqueSorted(envNames)
		result[len(result)-1].headerNames = uniqueSorted(headerNames)
	}
	return result, nil
}

func normalizeMCPServer(
	name,
	explicitType,
	command string,
	args []string,
	endpoint string,
	env,
	headers map[string]json.RawMessage,
	enabled bool,
) normalizedMCPServer {
	transport := strings.ToLower(strings.TrimSpace(explicitType))
	if transport == "" {
		switch {
		case command != "" && endpoint == "":
			transport = "stdio"
		case endpoint != "" && command == "":
			transport = "http"
		case command != "" && endpoint != "":
			transport = "ambiguous"
		default:
			transport = "unknown"
		}
	}
	if transport == "streamable-http" {
		transport = "http"
	}

	commandName := ""
	if command != "" {
		commandName = filepath.Base(filepath.Clean(command))
	}
	endpointHost := ""
	endpointScheme := ""
	if parsed, err := url.Parse(endpoint); err == nil {
		endpointScheme = strings.ToLower(parsed.Scheme)
		endpointHost = parsed.Hostname()
	}

	return normalizedMCPServer{
		name:           name,
		transport:      transport,
		command:        commandName,
		endpointScheme: endpointScheme,
		endpointHost:   endpointHost,
		envNames:       uniqueSorted(mapKeys(env)),
		headerNames:    uniqueSorted(mapKeys(headers)),
		enabled:        enabled,
		riskSignals:    mcpRiskSignals(commandName, args, endpoint, env, headers),
	}
}

func mcpArtifact(
	location mcpLocation,
	server normalizedMCPServer,
	options Options,
) (inventory.Artifact, *inventory.Diagnostic) {
	name := strings.TrimSpace(server.name)
	if name == "" {
		name = "unnamed"
	}
	metadata := map[string]string{
		"transport":     server.transport,
		"enabled":       fmt.Sprintf("%t", server.enabled),
		"config_format": location.format,
	}
	if server.command != "" {
		metadata["command"] = server.command
	}
	if server.endpointHost != "" {
		metadata["endpoint_host"] = server.endpointHost
	}
	if server.endpointScheme != "" {
		metadata["endpoint_scheme"] = server.endpointScheme
	}
	if len(server.envNames) != 0 {
		metadata["environment_variables"] = strings.Join(server.envNames, ",")
	}
	if len(server.headerNames) != 0 {
		metadata["header_names"] = strings.Join(server.headerNames, ",")
	}
	if len(location.consumers) != 0 {
		metadata["consumers"] = strings.Join(uniqueSorted(location.consumers), ",")
	}
	for key, value := range server.riskSignals {
		metadata[key] = value
	}

	artifact := inventory.Artifact{
		ID:       "mcp:" + location.provider + ":" + location.scope + ":" + name,
		Name:     name,
		Kind:     "mcp-server",
		Path:     location.path,
		Source:   location.provider,
		Scope:    location.scope,
		Metadata: metadata,
		Provenance: inventory.Provenance{
			Kind:     "configuration",
			Provider: location.provider,
			Package:  name,
		},
	}

	if validMCPTransport(server) {
		return artifact, nil
	}
	return artifact, &inventory.Diagnostic{
		Code:     "unsupported_transport",
		Level:    "warning",
		Provider: location.provider,
		Message:  fmt.Sprintf("MCP server %q does not declare exactly one supported command or URL", name),
		Path:     displayPath(location.path, options),
	}
}

func mcpRiskSignals(
	command string,
	args []string,
	endpoint string,
	env,
	headers map[string]json.RawMessage,
) map[string]string {
	signals := map[string]string{}
	lowerCommand := strings.ToLower(command)
	if containsString([]string{"sh", "bash", "zsh", "fish", "cmd", "cmd.exe", "powershell", "pwsh"}, lowerCommand) {
		signals["risk_shell_execution"] = "true"
	}
	if target, ok := packageExecutionTarget(lowerCommand, args); ok && !packageTargetPinned(target) {
		signals["risk_unpinned_package"] = "true"
	}
	if hasBroadFilesystemArgument(args) {
		signals["risk_broad_filesystem"] = "true"
	}
	sensitiveNames := sensitiveNames(mapKeys(env))
	if len(sensitiveNames) != 0 {
		signals["risk_sensitive_environment"] = strings.Join(sensitiveNames, ",")
	}
	if hasInlineCredentials(endpoint, env, headers) {
		signals["risk_inline_credentials"] = "true"
	}
	if parsed, err := url.Parse(endpoint); err == nil &&
		strings.EqualFold(parsed.Scheme, "http") &&
		parsed.Hostname() != "localhost" &&
		parsed.Hostname() != "127.0.0.1" &&
		parsed.Hostname() != "::1" {
		signals["risk_insecure_transport"] = "true"
	}
	return signals
}

func packageExecutionTarget(command string, args []string) (string, bool) {
	switch command {
	case "npx", "bunx", "uvx":
	case "pnpm", "yarn":
		if len(args) == 0 || args[0] != "dlx" {
			return "", false
		}
		args = args[1:]
	default:
		return "", false
	}
	for _, argument := range args {
		if !strings.HasPrefix(argument, "-") {
			return argument, true
		}
	}
	return "", false
}

func packageTargetPinned(target string) bool {
	if strings.Contains(target, "==") {
		return true
	}
	if strings.HasPrefix(target, "@") {
		return strings.LastIndex(target, "@") > 0
	}
	return strings.Contains(target, "@")
}

func hasBroadFilesystemArgument(args []string) bool {
	for _, argument := range args {
		clean := filepath.Clean(argument)
		if clean == string(filepath.Separator) || argument == "~" || argument == "$HOME" ||
			(len(argument) == 3 && argument[1:] == ":\\") {
			return true
		}
	}
	return false
}

func hasInlineCredentials(
	endpoint string,
	env,
	headers map[string]json.RawMessage,
) bool {
	if parsed, err := url.Parse(endpoint); err == nil {
		if parsed.User != nil {
			return true
		}
		for key := range parsed.Query() {
			if sensitiveName(key) {
				return true
			}
		}
	}
	for key, value := range headers {
		if sensitiveName(key) && rawLiteral(value) {
			return true
		}
	}
	for key, value := range env {
		if sensitiveName(key) && rawLiteral(value) {
			return true
		}
	}
	return false
}

func rawLiteral(value json.RawMessage) bool {
	var text string
	if json.Unmarshal(value, &text) != nil || strings.TrimSpace(text) == "" {
		return false
	}
	text = strings.TrimSpace(text)
	return !(strings.HasPrefix(text, "${") || strings.HasPrefix(text, "$env:"))
}

func sensitiveNames(names []string) []string {
	var result []string
	for _, name := range names {
		if sensitiveName(name) {
			result = append(result, name)
		}
	}
	return uniqueSorted(result)
}

func sensitiveName(name string) bool {
	upper := strings.ToUpper(name)
	for _, marker := range []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "API_KEY", "PRIVATE_KEY", "AUTHORIZATION"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func mapAsRaw[Value any](values map[string]Value) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		encoded, _ := json.Marshal(value)
		result[key] = encoded
	}
	return result
}

func validMCPTransport(server normalizedMCPServer) bool {
	switch server.transport {
	case "stdio":
		return server.command != "" && server.endpointHost == ""
	case "http", "sse":
		return server.command == "" && server.endpointHost != ""
	default:
		return false
	}
}

func samePath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftClean := filepath.Clean(leftAbsolute)
	rightClean := filepath.Clean(rightAbsolute)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftClean, rightClean)
	}
	return leftClean == rightClean
}

func codexEnvVarNames(values []any) []string {
	names := make([]string, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			names = append(names, typed)
		case map[string]any:
			if name, ok := typed["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

func mapKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := mapKeys(values)
	sort.Strings(keys)
	return keys
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
