package utils_test

import (
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// TestCreateBatches verifies that CreateBatches correctly partitions a flat slice
// of building IDs into fixed-size batches for parallel processing.
//
// Use case: City2TABULA distributes building IDs across worker goroutines in batches.
// Each batch becomes one Job in the JobQueue. The function must preserve element order,
// respect the batch size boundary, and handle edge cases gracefully.
func TestCreateBatches(t *testing.T) {
	cases := []struct {
		name      string
		ids       []int64
		batchSize int
		wantLen   int
		want      [][]int64
	}{
		{
			name:      "given a slice that divides evenly, when batched, then returns equal-sized batches",
			ids:       []int64{1, 2, 3, 4},
			batchSize: 2,
			wantLen:   2,
			want:      [][]int64{{1, 2}, {3, 4}},
		},
		{
			name:      "given a slice with a remainder, when batched, then last batch contains remaining elements",
			ids:       []int64{1, 2, 3, 4, 5},
			batchSize: 2,
			wantLen:   3,
			want:      [][]int64{{1, 2}, {3, 4}, {5}},
		},
		{
			name:      "given a slice is empty, when batched, then returns no batches",
			ids:       []int64{},
			batchSize: 2,
			wantLen:   0,
			want:      [][]int64{},
		},
		{
			name:      "given a batchSize is larger than total ids, when batched, then returns single batch with all ids",
			ids:       []int64{1, 2, 3, 4},
			batchSize: 5,
			wantLen:   1,
			want:      [][]int64{{1, 2, 3, 4}},
		},
		{
			name:      "given a negative batch size, when batched, then returns single batch with all ids",
			ids:       []int64{1, 2, 3, 4},
			batchSize: -1,
			wantLen:   1,
			want:      [][]int64{{1, 2, 3, 4}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {

			// When
			got := utils.CreateBatches(testCase.ids, testCase.batchSize)

			// Then: check correct number of batches
			if len(got) != testCase.wantLen {
				t.Fatalf("expected %d batches, got %d", testCase.wantLen, len(got))
			}

			// Then: each batch has correct length
			for i, batch := range testCase.want {
				if len(got[i]) != len(batch) {
					t.Errorf("batch[%d]: length = %d, want %d", i, len(got[i]), len(batch))
					continue
				}
				// Then: each batch content is in correct order
				for j, val := range batch {
					if got[i][j] != val {
						t.Errorf("batch[%d][%d] = %d, want %d", i, j, got[i][j], val)
					}
				}
			}
		})
	}
}

// BenchmarkCreateBatches measures how long CreateBatches takes to partition
// a slice of building IDs under different input sizes and batch sizes.
//
// Use case: CreateBatches runs once per pipeline execution on the full set of
// building IDs. Understanding its cost at different scales confirms it is not
// a bottleneck compared to the SQL execution it feeds into.
func BenchmarkCreateBatches(b *testing.B) {
	cases := []struct {
		name      string
		size      int
		batchSize int
	}{
		{"100_ids_batch10", 100, 10},
		{"1000_ids_batch100", 1000, 100},
		{"10000_ids_batch500", 10000, 500},
	}

	for _, bc := range cases {
		ids := make([]int64, bc.size)
		for i := range ids {
			ids[i] = int64(i + 1)
		}

		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				utils.CreateBatches(ids, bc.batchSize)
			}
		})
	}
}
