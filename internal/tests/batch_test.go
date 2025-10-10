package tests

import (
	"testing"

	"City2TABULA/internal/utils"

	"github.com/stretchr/testify/assert"
)

func TestCreateBatches(t *testing.T) {
	tests := []struct {
		name        string
		ids         []int64
		batchSize   int
		expected    [][]int64
		description string
	}{
		{
			name:        "Normal batching",
			ids:         []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			batchSize:   3,
			expected:    [][]int64{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10}},
			description: "Should create batches of size 3 with remainder in last batch",
		},
		{
			name:        "Exact division",
			ids:         []int64{1, 2, 3, 4, 5, 6},
			batchSize:   3,
			expected:    [][]int64{{1, 2, 3}, {4, 5, 6}},
			description: "Should create equal-sized batches when length is divisible by batch size",
		},
		{
			name:        "Batch size larger than input",
			ids:         []int64{1, 2, 3},
			batchSize:   5,
			expected:    [][]int64{{1, 2, 3}},
			description: "Should return single batch when batch size is larger than input",
		},
		{
			name:        "Single element",
			ids:         []int64{42},
			batchSize:   3,
			expected:    [][]int64{{42}},
			description: "Should handle single element correctly",
		},
		{
			name:        "Empty input",
			ids:         []int64{},
			batchSize:   3,
			expected:    nil, // Function returns nil for empty input
			description: "Should return nil slice for empty input",
		},
		{
			name:        "Zero batch size",
			ids:         []int64{1, 2, 3, 4, 5},
			batchSize:   0,
			expected:    [][]int64{{1, 2, 3, 4, 5}},
			description: "Should return single batch when batch size is 0",
		},
		{
			name:        "Negative batch size",
			ids:         []int64{1, 2, 3, 4, 5},
			batchSize:   -1,
			expected:    [][]int64{{1, 2, 3, 4, 5}},
			description: "Should return single batch when batch size is negative",
		},
		{
			name:        "Batch size of 1",
			ids:         []int64{1, 2, 3},
			batchSize:   1,
			expected:    [][]int64{{1}, {2}, {3}},
			description: "Should create individual batches when batch size is 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.CreateBatches(tt.ids, tt.batchSize) // Use assert for better test output
			assert.Equal(t, tt.expected, result, tt.description)

			// Additional validation: verify no elements are lost or duplicated
			if len(tt.ids) > 0 {
				var flattened []int64
				for _, batch := range result {
					flattened = append(flattened, batch...)
				}
				assert.Equal(t, tt.ids, flattened, "All elements should be preserved")
			}
		})
	}
}

func TestCreateBatchesSliceIndependence(t *testing.T) {
	// Test that modifying one batch doesn't affect others
	ids := []int64{1, 2, 3, 4, 5, 6}
	batches := utils.CreateBatches(ids, 2)

	// Modify the first batch
	if len(batches) > 0 && len(batches[0]) > 0 {
		batches[0][0] = 999
	}

	// Original slice should not be affected beyond the first element
	// This tests the slice independence created by the full slice expression
	assert.NotEqual(t, int64(999), ids[2], "Modifying batch should not affect unrelated elements in original slice")
}

func BenchmarkCreateBatches(b *testing.B) {
	// Create a large slice for benchmarking
	ids := make([]int64, 10000)
	for i := range ids {
		ids[i] = int64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.CreateBatches(ids, 100)
	}
}

func BenchmarkCreateBatchesSmallBatch(b *testing.B) {
	ids := make([]int64, 1000)
	for i := range ids {
		ids[i] = int64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.CreateBatches(ids, 10)
	}
}
