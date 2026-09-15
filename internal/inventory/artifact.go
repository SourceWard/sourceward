package inventory

type Artifact struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	Version    string            `json:"version,omitempty"`
	Path       string            `json:"path,omitempty"`
	LocalPath  string            `json:"-"`
	Source     string            `json:"source"`
	Scope      string            `json:"scope"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Provenance Provenance        `json:"provenance"`
}

type Provenance struct {
	Kind       string `json:"kind"`
	Repository string `json:"repository,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Dirty      *bool  `json:"dirty,omitempty"`
	Subdir     string `json:"subdirectory,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Publisher  string `json:"publisher,omitempty"`
	Package    string `json:"package,omitempty"`
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
