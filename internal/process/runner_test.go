package process

import (
	"errors"
	"testing"
)

// TestIsDeadLockError verifies that isDeadlockError correctly identifies PostgreSQL deadlock errors.
//
// Use case: RunTaskWithRetry uses this to decide whether to apply deadlock-specific retry logic
// instead of the general retry path.
func TestIsDeadLockError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "given error message containing 'deadlock detected', when called, then returns true",
			err:  errors.New("ERROR: deadlock detected"),
			want: true,
		},
		{
			name: "given error message containing 'sqlstate 40p01', when called, then returns true",
			err:  errors.New("ERROR: sqlstate 40p01"),
			want: true,
		},
		{
			name: "given error message not containing 'deadlock detected', when called, then returns false",
			err:  errors.New("ERROR: some other error"),
			want: false,
		},
		{
			name: "given nil error, when called, then returns false",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isDeadlockError(tc.err)

			if got != tc.want {
				t.Errorf("isDeadlockError(%v) = %v; want %v", tc.err, got, tc.want)
			}
		})
	}
}
