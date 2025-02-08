package model

// Of returns pointer to value.
func ptrOf[T any](value T) *T {
	return &value
}
