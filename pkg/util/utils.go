package util

// Ptr is a generic function that returns a pointer to whatever value is passed in
func Ptr[T any](v T) *T {
	return &v
}
