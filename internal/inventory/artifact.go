package inventory

type Artifact struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Kind     string            `json:"kind"`
	Version  string            `json:"version,omitempty"`
	Path     string            `json:"path,omitempty"`
	Source   string            `json:"source"`
	Scope    string            `json:"scope"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Level    string `json:"level"`
	Provider string `json:"provider"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

type Inventory struct {
	Artifacts   []Artifact   `json:"artifacts"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
