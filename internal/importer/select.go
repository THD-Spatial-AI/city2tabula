package importer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// fileExtent is the extent a model file declares, in the CRS it names.
type fileExtent struct {
	minX, minY, maxX, maxY float64
	crs                    string
}

var (
	// gmlEnvelope matches a CityGML boundedBy envelope; its corners may carry
	// attributes such as srsDimension.
	gmlEnvelope = regexp.MustCompile(`(?s)<gml:Envelope[^>]*\bsrsName="([^"]*)"[^>]*>\s*` +
		`<gml:lowerCorner[^>]*>([^<]*)</gml:lowerCorner>\s*<gml:upperCorner[^>]*>([^<]*)</gml:upperCorner>`)
	cityJSONMetadata   = regexp.MustCompile(`"metadata"\s*:\s*\{`)
	geographicalExtent = regexp.MustCompile(`"geographicalExtent"\s*:\s*\[([^\]]*)\]`)
)

// envelopeFirstRead is how much of a CityGML file is read before the rest of
// gmlHeaderBytes.
const envelopeFirstRead = 16 << 10

// metadataBytes is how much of a CityJSON file after the "metadata" key is
// searched for the dataset's geographicalExtent and referenceSystem.
const metadataBytes = 16 << 10

// readFileExtent returns the extent a model file declares: a CityGML file's
// boundedBy envelope in its first gmlHeaderBytes, or a CityJSON file's
// metadata.geographicalExtent and referenceSystem. ok is false when the file
// declares none, or none that can be read.
func readFileExtent(path, format string) (fileExtent, bool, error) {
	r, err := openModelFile(path)
	if err != nil {
		return fileExtent{}, false, err
	}
	defer r.Close()
	if format == "cityjson" {
		return cityJSONExtent(r)
	}
	// Most providers' envelope ends within the first few KiB, so the rest of
	// gmlHeaderBytes is read only when it is not found there.
	head, err := io.ReadAll(io.LimitReader(r, envelopeFirstRead))
	if err != nil {
		return fileExtent{}, false, err
	}
	m := gmlEnvelope.FindSubmatch(head)
	if m == nil && len(head) == envelopeFirstRead {
		rest, err := io.ReadAll(io.LimitReader(r, gmlHeaderBytes-envelopeFirstRead))
		if err != nil {
			return fileExtent{}, false, err
		}
		m = gmlEnvelope.FindSubmatch(append(head, rest...))
	}
	if m == nil {
		return fileExtent{}, false, nil
	}
	lower, upper := strings.Fields(string(m[2])), strings.Fields(string(m[3]))
	if len(lower) < 2 || len(upper) < 2 {
		return fileExtent{}, false, nil
	}
	return parseExtent(string(m[1]), lower[0], lower[1], upper[0], upper[1])
}

// cityJSONExtent streams r to its "metadata" object, which CityJSON writers put
// before or after CityObjects; a CityObject's own geographicalExtent comes
// earlier in some files, so only the one inside metadata counts.
func cityJSONExtent(r io.Reader) (fileExtent, bool, error) {
	br := bufio.NewReaderSize(r, 1<<20)
	var window []byte
	buf := make([]byte, 1<<20)
	for {
		n, readErr := io.ReadFull(br, buf)
		window = append(window, buf[:n]...)
		if loc := cityJSONMetadata.FindIndex(window); loc != nil {
			for len(window)-loc[0] < metadataBytes && readErr == nil {
				n, readErr = io.ReadFull(br, buf)
				window = append(window, buf[:n]...)
			}
			return extentFromMetadata(window[loc[0]:min(len(window), loc[0]+metadataBytes)])
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			return fileExtent{}, false, nil
		}
		if readErr != nil {
			return fileExtent{}, false, readErr
		}
		window = window[max(0, len(window)-4096):]
	}
}

func extentFromMetadata(metadata []byte) (fileExtent, bool, error) {
	e := geographicalExtent.FindSubmatch(metadata)
	crs := referenceSystemPattern.FindSubmatch(metadata)
	if e == nil || crs == nil {
		return fileExtent{}, false, nil
	}
	v := strings.Split(string(e[1]), ",")
	if len(v) != 6 { // minx, miny, minz, maxx, maxy, maxz
		return fileExtent{}, false, nil
	}
	return parseExtent(string(crs[1]), v[0], v[1], v[3], v[4])
}

func parseExtent(crs string, values ...string) (fileExtent, bool, error) {
	var f [4]float64
	for i, s := range values {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return fileExtent{}, false, nil
		}
		f[i] = v
	}
	return fileExtent{minX: f[0], minY: f[1], maxX: f[2], maxY: f[3], crs: crs}, true, nil
}

// selectFiles returns, per model folder of d, the files whose declared extent
// intersects b, and their fingerprint. A file whose extent or CRS cannot be
// read is selected, so citydb-tool's own bbox filter decides for it.
func selectFiles(ctx context.Context, conn *pgxpool.Pool, d Dataset, b importBbox) (map[string][]string, string, error) {
	inEPSG := map[int]importBbox{}
	selection := map[string][]string{}
	var all []string
	total := 0
	for _, in := range d.Inputs {
		files, err := modelFiles(in.path)
		if err != nil {
			return nil, "", err
		}
		total += len(files)
		for _, path := range files {
			keep, err := intersects(ctx, conn, path, in.format, b, inEPSG)
			if err != nil {
				return nil, "", err
			}
			if keep {
				selection[in.path] = append(selection[in.path], path)
				all = append(all, path)
			}
		}
	}
	utils.Info.Printf("%s: %d of %d files intersect the bbox", d.Attribution.DatasetID, len(all), total)
	fingerprint, err := fingerprintFiles(all)
	return selection, fingerprint, err
}

// intersects reports whether path's declared extent intersects b, transforming
// b once per CRS into inEPSG.
func intersects(ctx context.Context, conn *pgxpool.Pool, path, format string, b importBbox, inEPSG map[int]importBbox) (bool, error) {
	e, ok, err := readFileExtent(path, format)
	if err != nil {
		return false, fmt.Errorf("read extent of %s: %w", path, err)
	}
	if !ok {
		return true, nil
	}
	epsg, err := resolveCRS(e.crs)
	if err != nil {
		return true, nil
	}
	t, done := inEPSG[epsg]
	if !done {
		if err := conn.QueryRow(ctx, `
			SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
			FROM (SELECT ST_Transform(ST_MakeEnvelope($1, $2, $3, $4, $5::int), $6::int) AS e) s`,
			b.xmin, b.ymin, b.xmax, b.ymax, b.srid, epsg,
		).Scan(&t.xmin, &t.ymin, &t.xmax, &t.ymax); err != nil {
			return false, fmt.Errorf("transform bbox to EPSG:%d for %s: %w", epsg, path, err)
		}
		inEPSG[epsg] = t
	}
	return e.minX <= t.xmax && e.maxX >= t.xmin && e.minY <= t.ymax && e.maxY >= t.ymin, nil
}

// modelFiles lists the model files under one model folder, in path order.
func modelFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !isModelFile(path) {
			return err
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// writeArgFile writes paths to a citydb-tool argument file, one quoted path per
// line, and returns its name.
func writeArgFile(paths []string) (string, error) {
	f, err := os.CreateTemp("", "c2t-import-*.args")
	if err != nil {
		return "", err
	}
	defer f.Close()
	for _, p := range paths {
		quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(p) + `"`
		if _, err := fmt.Fprintln(f, quoted); err != nil {
			return "", err
		}
	}
	return f.Name(), nil
}
