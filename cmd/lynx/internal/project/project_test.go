package project

import (
	"reflect"
	"testing"
)

func TestCheckDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "simple names",
			input:    []string{"foo", "bar", "foo", "baz"},
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "path-like names allowed",
			input:    []string{"foo/bar/svc", "a/b"},
			expected: []string{"foo/bar/svc", "a/b"},
		},
		{
			name:     "mixed path and simple",
			input:    []string{"mysvc", "team/mysvc", "mysvc"},
			expected: []string{"mysvc", "team/mysvc"},
		},
		{
			name:     "invalid simple name filtered",
			input:    []string{"valid", "in valid", "valid", "also-valid"},
			expected: []string{"valid", "also-valid"},
		},
		{
			name:     "trim space and empty",
			input:    []string{"  a  ", "a", "", "  b  "},
			expected: []string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkDuplicates(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("checkDuplicates() = %v, want %v", got, tt.expected)
			}
		})
	}
}
