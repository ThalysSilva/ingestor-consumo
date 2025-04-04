package utils

import "fmt"

func Retry(fn func() error, retries int) error {
	for range retries {
		if err := fn(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("falha após %d tentativas", retries)
}
