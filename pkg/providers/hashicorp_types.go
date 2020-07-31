package providers

type hashiCorpRelease struct {
	Name             string           `json:"name"`
	Version          string           `json:"version"`
	Shasums          string           `json:"shasums"`
	ShasumsSignature string           `json:"shasums_signature"`
	Builds           []hashiCorpBuild `json:"builds"`
}

type hashiCorpRepo struct {
	Name     string                      `json:"name"`
	Versions map[string]hashiCorpVersion `json:"versions"`
}

type hashiCorpVersion struct {
	Name             string           `json:"name"`
	Version          string           `json:"version"`
	Shasums          string           `json:"shasums"`
	ShasumsSignature string           `json:"shasums_signature"`
	Builds           []hashiCorpBuild `json:"builds"`
}

type hashiCorpBuild struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}
