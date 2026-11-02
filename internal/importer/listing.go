package importer

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// DatasetRegion is one region folder of a dataset's model folder or, for a
// dataset without regions.json, the whole model folder. Region, Name and Bbox
// are empty in that case.
type DatasetRegion struct {
	Country   string    `json:"country"`
	DatasetID string    `json:"dataset_id"`
	Provider  string    `json:"provider"`
	LOD       int       `json:"lod"`
	Format    string    `json:"format"`
	Region    string    `json:"region,omitempty"`
	Name      string    `json:"name,omitempty"`
	Bbox      []float64 `json:"bbox,omitempty"`
	Files     int       `json:"files"`
}

// ListDatasets returns the region folders of every dataset under cfg's LOD2
// and LOD3 data directories. It reads files only. A dataset folder that the
// import or -check-data would refuse for its attribution or regions.json is an
// error naming the file.
func ListDatasets(cfg *config.Config) ([]DatasetRegion, error) {
	datasets, err := DiscoverDatasets(cfg)
	if err != nil {
		return nil, err
	}
	var list []DatasetRegion
	for _, d := range datasets {
		if problems := checkRegions(d.Dir, d.Inputs); len(problems) > 0 {
			return nil, errors.Join(problems...)
		}
		r, hasRegions, err := loadRegions(d.Dir)
		if err != nil {
			return nil, err
		}
		base := DatasetRegion{Country: cfg.Country, DatasetID: d.Attribution.DatasetID, Provider: d.Attribution.Provider, LOD: 2}
		if d.Schema == cfg.DB.Schemas.Lod3 {
			base.LOD = 3
		}
		for _, in := range d.Inputs {
			base.Format = in.format
			found, err := folderRegions(base, in.path, r, hasRegions)
			if err != nil {
				return nil, err
			}
			list = append(list, found...)
		}
	}
	return list, nil
}

// folderRegions lists the region folders under one model folder, or the model
// folder itself when the dataset has no regions file.
func folderRegions(base DatasetRegion, folder string, r regionsFile, hasRegions bool) ([]DatasetRegion, error) {
	if !hasRegions {
		files, err := modelFiles(folder)
		base.Files = len(files)
		return []DatasetRegion{base}, err
	}
	var list []DatasetRegion
	for _, slug := range slices.Sorted(maps.Keys(r.Regions)) {
		dir := filepath.Join(folder, slug)
		if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			continue // the region's files are in the dataset's other model folder
		}
		files, err := modelFiles(dir)
		if err != nil {
			return nil, err
		}
		e := base
		e.Region, e.Name, e.Bbox, e.Files = slug, r.Regions[slug].Name, r.Regions[slug].Bbox, len(files)
		list = append(list, e)
	}
	return list, nil
}
