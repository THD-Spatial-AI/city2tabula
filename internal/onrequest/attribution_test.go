package onrequest

import (
	"slices"
	"testing"
)

func TestBuildingDatasetIDs(t *testing.T) {
	code := "DE.N.SFH.01.Gen.ReEx.001.001"
	tests := []struct {
		name      string
		buildings []Building
		want      []string
	}{
		{"none", nil, nil},
		{"one dataset, no TABULA type", []Building{{DatasetID: "de-a-lod2"}, {DatasetID: "de-a-lod2"}}, []string{"de-a-lod2"}},
		{"TABULA type adds TABULA", []Building{{DatasetID: "de-a-lod2", TabulaVariantCode: &code}}, []string{"de-a-lod2", "tabula-episcope"}},
		{"two datasets, sorted", []Building{{DatasetID: "nl-b-lod2"}, {DatasetID: "de-a-lod2"}}, []string{"de-a-lod2", "nl-b-lod2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildingDatasetIDs(tc.buildings); !slices.Equal(got, tc.want) {
				t.Errorf("BuildingDatasetIDs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGeometryDatasetIDs(t *testing.T) {
	got := GeometryDatasetIDs([]BuildingGeometry{{DatasetID: "nl-b-lod2"}, {DatasetID: "de-a-lod2"}, {DatasetID: "nl-b-lod2"}})
	if want := []string{"de-a-lod2", "nl-b-lod2"}; !slices.Equal(got, want) {
		t.Errorf("GeometryDatasetIDs = %v, want %v", got, want)
	}
}
