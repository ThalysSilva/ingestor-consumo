package utils

import (
	"fmt"
	"testing"
)

func TestRetry(t *testing.T) {

	t.Run("Success", func(t *testing.T) {
		count := 0
		err := Retry(func() error {
			count++
			if count < 2 {
				return fmt.Errorf("erro")
			}
			return nil
		}, 3)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if count != 2 {
			t.Fatalf("expected count to be 2, got %d", count)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		count := 0
		err := Retry(func() error {
			count++
			return fmt.Errorf("erro")
		}, 3)

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if count != 3 {
			t.Fatalf("expected count to be 3, got %d", count)
		}
	})

}
