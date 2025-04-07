package utils

import "testing"

/* func ChunkSlice[T any](slice []T, chunkSize int) [][]T {
	var chunks [][]T
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// chunkMapValues converte os valores de um map em um slice e os divide em lotes de tamanho fixo.
func ChunkMapValues[K comparable, V any](m map[K]V, chunkSize int) [][]V {
	values := make([]V, 0, len(m))
	for _, value := range m {
		values = append(values, value)
	}

	return ChunkSlice(values, chunkSize)
} */

func TestChunkSlice(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	chunkSize := 2
	expected := [][]int{{1, 2}, {3, 4}, {5}}
	result := ChunkSlice(slice, chunkSize)

	if len(result) != len(expected) {
		t.Errorf("expected %d chunks, got %d", len(expected), len(result))
	}

	for i := range result {
		if len(result[i]) != len(expected[i]) {
			t.Errorf("expected chunk size %d, got %d", len(expected[i]), len(result[i]))
		}
		for j := range result[i] {
			if result[i][j] != expected[i][j] {
				t.Errorf("expected value %d at chunk %d index %d, got %d", expected[i][j], i, j, result[i][j])
			}
		}
	}
}

func TestChunkMapValues(t *testing.T) {
	m := map[string]int{
		"key1": 1,
		"key2": 2,
		"key3": 3,
	}
	chunkSize := 2
	expected := [][]int{{1, 2}, {3}}
	result := ChunkMapValues(m, chunkSize)

	if len(result) != len(expected) {
		t.Errorf("expected %d chunks, got %d", len(expected), len(result))
	}

	for i := range result {
		if len(result[i]) != len(expected[i]) {
			t.Errorf("expected chunk size %d, got %d", len(expected[i]), len(result[i]))
		}
		for j := range result[i] {
			if result[i][j] != expected[i][j] {
				t.Errorf("expected value %d at chunk %d index %d, got %d", expected[i][j], i, j, result[i][j])
			}
		}
	}
}
