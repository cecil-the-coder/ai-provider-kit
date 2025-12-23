package utils

import "testing"

func TestMin(t *testing.T) {
	t.Run("AIsSmaller", func(t *testing.T) {
		result := Min(5, 10)
		if result != 5 {
			t.Errorf("Expected 5, got %d", result)
		}
	})

	t.Run("BIsSmaller", func(t *testing.T) {
		result := Min(10, 5)
		if result != 5 {
			t.Errorf("Expected 5, got %d", result)
		}
	})

	t.Run("Equal", func(t *testing.T) {
		result := Min(5, 5)
		if result != 5 {
			t.Errorf("Expected 5, got %d", result)
		}
	})

	t.Run("NegativeNumbers", func(t *testing.T) {
		result := Min(-10, -5)
		if result != -10 {
			t.Errorf("Expected -10, got %d", result)
		}
	})

	t.Run("Zero", func(t *testing.T) {
		result := Min(0, 5)
		if result != 0 {
			t.Errorf("Expected 0, got %d", result)
		}
	})
}
