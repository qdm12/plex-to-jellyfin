package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_New(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		libraries []string
		expected  []string
	}{
		"empty_input": {
			libraries: nil,
			expected:  []string{},
		},
		"case_normalized": {
			libraries: []string{"Movies ", "TV Shows"},
			expected:  []string{"movies", "tv shows"},
		},
		"deduplicates_and_sorts": {
			libraries: []string{"Z", "a", "A", "z", " a "},
			expected:  []string{"a", "z"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			migrator := New(nil, nil, nil, "jellyfin-user", testCase.libraries, false)

			assert.Equal(t, testCase.expected, migrator.libraries)
		})
	}
}
