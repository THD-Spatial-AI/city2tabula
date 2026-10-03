// Package attribution reads and validates attribution.json, the file that
// credits the provider of one source dataset. Every dataset folder City2TABULA
// imports carries one at its root. The format is documented in
// docs/code/attribution/index.md and published as a JSON Schema in
// docs/code/attribution/attribution.schema.json, which a test keeps in step
// with the Attribution struct.
package attribution

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileName is the attribution file's name at the root of a dataset folder.
const FileName = "attribution.json"

// SchemaVersion is the attribution file format this package reads.
const SchemaVersion = 1

// TabulaDatasetID is the dataset_id data/tabula/attribution.json must carry.
// The API attaches this dataset's credit wherever it returns a TABULA type.
const TabulaDatasetID = "tabula-episcope"

// Attribution is the content of one attribution.json file.
type Attribution struct {
	SchemaVersion int    `json:"schema_version"`
	DatasetID     string `json:"dataset_id"`
	Provider      string `json:"provider"`
	Dataset       string `json:"dataset"`
	Licence       string `json:"licence"`
	LicenceURL    string `json:"licence_url"`
	Credit        string `json:"credit"`
	CreditURL     string `json:"credit_url,omitempty"`
	TermsURL      string `json:"terms_url,omitempty"`
	Changes       string `json:"changes"`
}

var (
	datasetIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// An SPDX licence id (CC-BY-4.0, DL-DE-BY-2.0, GPL-2.0+) or an SPDX
	// LicenseRef-<id> for a licence not on the SPDX list. Membership in the
	// SPDX list is not checked.
	licencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*\+?$`)
)

// Load reads and validates the attribution file in dir. Every error names the
// file, so a failed run points at the dataset folder to fix.
func Load(dir string) (Attribution, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return Attribution{}, fmt.Errorf("read attribution file: %w", err)
	}
	a, err := decode(data)
	if err != nil {
		return Attribution{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := a.Validate(); err != nil {
		return Attribution{}, fmt.Errorf("%s: %w", path, err)
	}
	return a, nil
}

// decode parses one JSON object, rejecting unknown fields so a misspelt key
// fails instead of leaving its field empty.
func decode(data []byte) (Attribution, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var a Attribution
	if err := dec.Decode(&a); err != nil {
		return Attribution{}, fmt.Errorf("parse JSON: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Attribution{}, errors.New("parse JSON: unexpected data after the attribution object")
	}
	return a, nil
}

// Validate reports every rule a violates in one error, so the author of a
// file sees all fixes at once.
func (a Attribution) Validate() error {
	var errs []error
	if a.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version is %d, want %d", a.SchemaVersion, SchemaVersion))
	}
	if !datasetIDPattern.MatchString(a.DatasetID) {
		errs = append(errs, fmt.Errorf("dataset_id %q must be lowercase letters and digits in hyphen-separated words, e.g. cz-brno-lod2", a.DatasetID))
	}
	for _, f := range []struct{ name, value string }{
		{"provider", a.Provider}, {"dataset", a.Dataset}, {"credit", a.Credit}, {"changes", a.Changes},
	} {
		if strings.TrimSpace(f.value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", f.name))
		}
	}
	if !licencePattern.MatchString(a.Licence) {
		errs = append(errs, fmt.Errorf("licence %q must be an SPDX licence id, e.g. CC-BY-4.0, or LicenseRef-<id>", a.Licence))
	}
	errs = append(errs, checkURL("licence_url", a.LicenceURL, true))
	errs = append(errs, checkURL("credit_url", a.CreditURL, false))
	errs = append(errs, checkURL("terms_url", a.TermsURL, false))
	return errors.Join(errs...)
}

// checkURL requires an absolute http or https URL. An empty optional field
// passes.
func checkURL(field, raw string, mandatory bool) error {
	if raw == "" {
		if mandatory {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s %q must be an absolute http or https URL", field, raw)
	}
	return nil
}
