package generation

import "github.com/go-redis/redis/v8"

// GetCurrentGeneration obtém a geração atual do Redis
// Se a chave não existir, cria uma nova chave com o valor "A"
// Retorna a geração atual e um erro, se houver
func (m *managerGeneration) GetCurrentGeneration() (string, error) {
	gen, err := m.redisClient.Get(m.ctx, "current_generation").Result()
	if err == redis.Nil {
		if err := m.redisClient.Set(m.ctx, "current_generation", "A", 0).Err(); err != nil {
			return "", err
		}
		return "A", nil
	} else if err != nil {
		return "", err
	}
	return gen, nil
}

// ToggleGeneration alterna a geração atual entre "A" e "B"
// Atualiza a chave "current_generation" no Redis com o novo valor
// Retorna a nova geração e um erro, se houver
func (m *managerGeneration) ToggleGeneration() (nextGen string, err error) {
	currentGen := m.redisClient.Get(m.ctx, "current_generation").Val()

	nextGen = "B"
	if currentGen == "B" {
		nextGen = "A"
	}

	if err := m.redisClient.Set(m.ctx, "current_generation", nextGen, 0).Err(); err != nil {
		return "", err
	}
	return nextGen, nil
}
