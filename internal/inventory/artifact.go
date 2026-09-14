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

type Inventory struct {
	Artifacts []Artifact `json:"artifacts"`
}
