// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package model

// Of returns pointer to value.
func ptrOf[T any](value T) *T {
	return &value
}
