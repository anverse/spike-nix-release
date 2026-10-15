// Package version is the shared contract of the spike's services: what
// they report at GET /version. It stands in for agentd's Session API.
package version

// Info describes a running component.
type Info struct {
	Component string `json:"component"`
	Version   string `json:"version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}
