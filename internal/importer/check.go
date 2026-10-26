package importer

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// gmlHeaderBytes is how much of a CityGML file is searched for its root element
// and srsName; providers declare the CRS on the first envelope or geometry.
const gmlHeaderBytes = 1 << 20

var (
	srsNamePattern         = regexp.MustCompile(`srsName="([^"]*)"`)
	referenceSystemPattern = regexp.MustCompile(`"referenceSystem"\s*:\s*"([^"]*)"`)
	// epsgName matches the CRS name forms citydb-tool reads as an EPSG code:
	// urn:ogc:def:crs:EPSG::<code> (also the compound urn:ogc:def:crs,crs:EPSG::...),
	// the OGC http(s) URI and EPSG:<code>. The first code is the horizontal CRS.
	epsgName = regexp.MustCompile(`^(?:urn:ogc:def:crs(?:,crs)?:EPSG:[^:]*:|https?://www\.opengis\.net/def/crs/EPSG/0/|EPSG:)(\d+)`)
)

// advCRSPrefixes maps the German AdV CRS names, which continue with a height
// system (e.g. "*DE_DHHN2016_NH"), to their horizontal EPSG code.
var advCRSPrefixes = map[string]int{
	"urn:adv:crs:ETRS89_UTM32": 25832,
	"urn:adv:crs:ETRS89_UTM33": 25833,
}

// compoundEPSG maps compound EPSG codes to their horizontal component.
var compoundEPSG = map[int]int{
	7415: 28992, // Amersfoort / RD New + NAP height
}

// resolveCRS returns the horizontal EPSG code a declared CRS name stands for.
func resolveCRS(name string) (int, error) {
	for prefix, code := range advCRSPrefixes {
		if strings.HasPrefix(name, prefix) {
			return code, nil
		}
	}
	m := epsgName.FindStringSubmatch(name)
	if m == nil {
		return 0, fmt.Errorf("CRS %q is not a recognised EPSG or AdV name", name)
	}
	code, _ := strconv.Atoi(m[1]) // \d+ always parses
	if horizontal, ok := compoundEPSG[code]; ok {
		return horizontal, nil
	}
	return code, nil
}

// CheckDataset checks a staged dataset folder for publishing under cfg's country
// without importing it or touching a database: the rules the import applies
// (attribution.json, model folders, a dataset_id that clashes with no published
// folder of other content) plus each model file's format and declared CRS, which
// must be the SRID the import uses: CITYDB_SRID, or the country's default. It
// returns every problem, each naming its path.
func CheckDataset(cfg *config.Config, dir string) []error {
	sridText, _, err := config.SRIDForCountry(cfg.Country)
	if err != nil {
		return []error{err}
	}
	if cfg.CityDB != nil && cfg.CityDB.SRID != "" {
		sridText = cfg.CityDB.SRID
	}
	srid, err := strconv.Atoi(sridText)
	if err != nil {
		return []error{fmt.Errorf("SRID %q is not an EPSG code", sridText)}
	}

	var problems []error
	a, err := attribution.Load(dir)
	if err != nil {
		problems = append(problems, err)
	} else if err := checkDatasetIDFree(cfg, dir, a); err != nil {
		problems = append(problems, err)
	}
	problems = append(problems, strayModelFiles(dir)...)
	inputs, err := datasetInputs(dir)
	if err != nil {
		return append(problems, err)
	}
	for _, in := range inputs {
		problems = append(problems, checkModelFolder(in, srid)...)
	}
	return problems
}

// checkDatasetIDFree refuses a dataset_id that a published folder already uses
// with a different attribution. The same attribution means the staged files
// extend that dataset.
func checkDatasetIDFree(cfg *config.Config, dir string, a attribution.Attribution) error {
	published, err := DiscoverDatasets(cfg)
	if err != nil {
		return fmt.Errorf("published data for %s cannot be read: %w", cfg.Country, err)
	}
	staged, _ := filepath.Abs(dir)
	for _, d := range published {
		other, _ := filepath.Abs(d.Dir)
		if d.Attribution.DatasetID == a.DatasetID && other != staged && d.Attribution != a {
			return fmt.Errorf("%s: dataset_id %q is already used by %s with a different attribution.json", dir, a.DatasetID, d.Dir)
		}
	}
	return nil
}

// strayModelFiles reports model files outside gml/ and cityjson/, which an
// import would not read. Hidden entries are skipped, as the import skips them.
func strayModelFiles(dir string) []error {
	var problems []error
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		top := strings.Split(rel, string(filepath.Separator))[0]
		if e.IsDir() && path != dir && (top == "gml" || top == "cityjson" || strings.HasPrefix(e.Name(), ".")) {
			return fs.SkipDir
		}
		if strings.HasPrefix(e.Name(), ".") {
			return nil
		}
		if !e.IsDir() && rel != attribution.FileName && isModelFile(path) {
			problems = append(problems, fmt.Errorf("%s lies outside gml/ and cityjson/ and would not be imported", path))
		}
		return nil
	})
	if err != nil {
		problems = append(problems, fmt.Errorf("read %s: %w", dir, err))
	}
	return problems
}

// isModelFile reports whether path has a model file extension, gzipped or not.
func isModelFile(path string) bool {
	return modelExtensions[strings.ToLower(filepath.Ext(strings.TrimSuffix(strings.ToLower(path), ".gz")))]
}

// checkModelFolder checks every model file under one gml/ or cityjson/ folder.
func checkModelFolder(in importInput, srid int) []error {
	var problems []error
	err := filepath.WalkDir(in.path, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !isModelFile(path) {
			return err
		}
		check := checkCityGMLFile
		if in.format == "cityjson" {
			check = checkCityJSONFile
		}
		if err := check(path, srid); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", path, err))
		}
		return nil
	})
	if err != nil {
		problems = append(problems, fmt.Errorf("read %s: %w", in.path, err))
	}
	return problems
}

// openModelFile opens path, decompressing a .gz file.
func openModelFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".gz") {
		return f, nil
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("not a readable gzip file: %w", err)
	}
	return struct {
		io.Reader
		io.Closer
	}{gz, f}, nil
}

// checkCityGMLFile checks that a file is CityGML and that every srsName in its
// first gmlHeaderBytes resolves to srid.
func checkCityGMLFile(path string, srid int) error {
	r, err := openModelFile(path)
	if err != nil {
		return err
	}
	defer r.Close()
	head, err := io.ReadAll(io.LimitReader(r, gmlHeaderBytes))
	if err != nil {
		return fmt.Errorf("cannot be read: %w", err)
	}
	if !strings.Contains(string(head), "CityModel") {
		return errors.New("is not CityGML: no CityModel element at its start")
	}
	names := srsNamePattern.FindAllStringSubmatch(string(head), -1)
	if len(names) == 0 {
		return fmt.Errorf("declares no srsName in its first %d bytes", gmlHeaderBytes)
	}
	seen := map[string]bool{}
	for _, m := range names {
		if !seen[m[1]] {
			seen[m[1]] = true
			if err := checkCRS(m[1], srid); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkCityJSONFile checks that a file is CityJSON and that its
// metadata.referenceSystem resolves to srid. The whole file is scanned, as a
// CityJSON writer may put metadata after CityObjects.
func checkCityJSONFile(path string, srid int) error {
	r, err := openModelFile(path)
	if err != nil {
		return err
	}
	defer r.Close()
	isCityJSON, referenceSystem, err := scanCityJSON(r)
	if err != nil {
		return fmt.Errorf("cannot be read: %w", err)
	}
	if !isCityJSON {
		return errors.New(`is not CityJSON: no "type":"CityJSON"`)
	}
	if referenceSystem == "" {
		return errors.New("declares no metadata.referenceSystem")
	}
	return checkCRS(referenceSystem, srid)
}

// scanCityJSON streams r in chunks that overlap by more than a match, so a
// pattern split across two reads is still found.
func scanCityJSON(r io.Reader) (isCityJSON bool, referenceSystem string, err error) {
	const chunk, overlap = 1 << 20, 4096
	br := bufio.NewReaderSize(r, chunk)
	var tail []byte
	buf := make([]byte, chunk)
	for {
		n, readErr := io.ReadFull(br, buf)
		window := append(tail, buf[:n]...)
		isCityJSON = isCityJSON || strings.Contains(string(window), `"type":"CityJSON`) || strings.Contains(string(window), `"type": "CityJSON`)
		if m := referenceSystemPattern.FindSubmatch(window); m != nil && referenceSystem == "" {
			referenceSystem = string(m[1])
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			return isCityJSON, referenceSystem, nil
		}
		if readErr != nil {
			return false, "", readErr
		}
		tail = append([]byte(nil), window[max(0, len(window)-overlap):]...)
	}
}

// checkCRS refuses a declared CRS that does not resolve to srid.
func checkCRS(name string, srid int) error {
	code, err := resolveCRS(name)
	if err != nil {
		return err
	}
	if code != srid {
		return fmt.Errorf("declares CRS %q (EPSG:%d), but this country's data must be in EPSG:%d", name, code, srid)
	}
	return nil
}
