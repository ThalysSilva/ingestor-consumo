package utils

func ChunkSlice[T any](slice []T, chunkSize int) [][]T {
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
}
