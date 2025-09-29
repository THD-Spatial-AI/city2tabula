package utils

func CreateBatches(ids []int64, batchSize int) [][]int64 {
	if batchSize <= 0 {
		return [][]int64{ids} // No batching, return single batch
	}

	var batches [][]int64
	for batchSize < len(ids) {
		// Use full slice expression to create a new slice for each batch
		ids, batches = ids[batchSize:], append(batches, ids[0:batchSize:batchSize])
	}
	if len(ids) > 0 {
		batches = append(batches, ids)
	}
	return batches
}
