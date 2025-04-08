package utils

import (
	"reflect"
	"sort"
	"testing"
)

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
	result := ChunkMapValues(m, chunkSize)

	// Junta todos os chunks em uma lista só
	var flat []int
	for _, chunk := range result {
		flat = append(flat, chunk...)
	}

	// Ordena pra comparar sem se importar com ordem
	sort.Ints(flat)
	expected := []int{1, 2, 3}

	if !reflect.DeepEqual(flat, expected) {
		t.Errorf("expected flattened values %v, got %v", expected, flat)
	}

	// Testar se os chunks têm tamanho máximo correto
	for _, chunk := range result {
		if len(chunk) > chunkSize {
			t.Errorf("chunk size exceeds limit: %v", chunk)
		}
	}
}
