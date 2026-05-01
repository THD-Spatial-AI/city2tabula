package utils_test

import (
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// TestCreateBatches_ExactDivision tests CreateBatches with a slice that divides evenly into batches
func TestCreateBatches_ExactDivision(t *testing.T) {

	got := utils.CreateBatches([]int64{1, 2, 3, 4}, 2)

	if len(got) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(got))
	}

	// Fatalf above guards us — if we reach here, got[0] and got[1] are safe to access
	if got[0][0] != 1 || got[0][1] != 2 {
		t.Errorf("batch[0] = %v, want [1 2]", got[0])
	}
	if got[1][0] != 3 || got[1][1] != 4 {
		t.Errorf("batch[1] = %v, want [3 4]", got[1])
	}
}

func TestCreateBatches_WithRemainder(t *testing.T) {
	got := utils.CreateBatches([]int64{1, 2, 3, 4, 5}, 2)

	if len(got) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(got))
	}

	// Fatalf above guards us - if we reach here, got[0], got[1], and got [2] are safe to access
	if got[0][0] != 1 || got[0][1] != 2 {
		t.Errorf("batch[0] = %v, want [1 2]", got[0])
	}
	if got[1][0] != 3 || got[1][1] != 4 {
		t.Errorf("batch[1] = %v, want [3 4]", got[1])
	}
	if len(got[2]) != 1 || got[2][0] != 5 {
		t.Errorf("batch[2] = %v, want [5]", got[2])
	}
}

func TestCreateBatches_EmptyInput(t *testing.T) {
	got := utils.CreateBatches([]int64{}, 2)

	if len(got) != 0 {
		t.Fatalf("expected 0 batches, got %d", len(got))
	}
}

func TestCreateBatches_BatchSizeLargerThanInput(t *testing.T) {
	got := utils.CreateBatches([]int64{1, 2, 3, 4}, 10)

	if len(got) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(got))
	}

	if len(got[0]) != 4 || got[0][0] != 1 || got[0][1] != 2 || got[0][2] != 3 || got[0][3] != 4 {
		t.Errorf("batch[0] = %v, want [1 2 3 4]", got[0])
	}
}
