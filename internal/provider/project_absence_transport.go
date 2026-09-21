package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// projectAbsenceTransport disambiguates project-scoped 404 responses before
// either generated or native EX clients can interpret them as object deletion.
// Semaphore also uses 404 when a non-admin loses project membership.
type projectAbsenceTransport struct {
	next     http.RoundTripper
	basePath string
}

func (t *projectAbsenceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	response, err := t.next.RoundTrip(req)
	if err != nil || response.StatusCode != http.StatusNotFound || (req.Method != http.MethodGet && req.Method != http.MethodDelete) {
		return response, err
	}
	prefix := strings.TrimRight(t.basePath, "/") + "/project/"
	if !strings.HasPrefix(req.URL.Path, prefix) {
		return response, nil
	}
	parts := strings.Split(strings.TrimPrefix(req.URL.Path, prefix), "/")
	projectID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || projectID <= 0 {
		return t.unconfirmed(req, response)
	}
	projectPath := prefix + parts[0]
	if strings.TrimRight(req.URL.Path, "/") != projectPath {
		var project struct {
			ID int64 `json:"id"`
		}
		status, probeErr := t.probe(req, projectPath, &project)
		if probeErr == nil && status == http.StatusOK && project.ID == projectID {
			return response, nil
		}
		if probeErr != nil || status != http.StatusNotFound {
			return t.unconfirmed(req, response)
		}
	}
	// Only an administrator can distinguish a deleted project from lost access.
	// Check on every ambiguous response; caching authority would hide revocation.
	var user struct {
		ID    int64 `json:"id"`
		Admin *bool `json:"admin"`
	}
	status, probeErr := t.probe(req, strings.TrimRight(t.basePath, "/")+"/user", &user)
	if probeErr == nil && status == http.StatusOK && user.ID > 0 && user.Admin != nil && *user.Admin {
		return response, nil
	}
	return t.unconfirmed(req, response)
}

// Probes use the same origin, authentication, TLS transport and cancellation as
// the failed request. Calling the underlying transport avoids recursive probes
// and deliberately does not follow redirects to another authority.
func (t *projectAbsenceTransport) probe(original *http.Request, path string, result any) (int, error) {
	target := *original.URL
	target.Path = path
	target.RawPath = ""
	target.RawQuery = ""
	target.Fragment = ""
	req, err := http.NewRequestWithContext(original.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header = original.Header.Clone()
	req.Header.Set("Accept", "application/json")
	response, err := t.next.RoundTrip(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if err := decoder.Decode(result); err != nil {
		return response.StatusCode, err
	}
	return response.StatusCode, nil
}

func (t *projectAbsenceTransport) unconfirmed(req *http.Request, response *http.Response) (*http.Response, error) {
	_ = response.Body.Close()
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	return nil, &exTransportError{message: "Project resource absence cannot be confirmed. Semaphore returns 404 for both missing projects and revoked project access. Restore project access or verify the API endpoint and retry; Terraform state is retained. If an inaccessible object is independently confirmed deleted, remove its state explicitly."}
}
