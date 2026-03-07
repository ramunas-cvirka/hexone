package windowstate

import "testing"

func TestSessionWindowPositionLooksHidden(t *testing.T) {
	tests := []struct {
		x, y int
		want bool
	}{
		{x: -32000, y: -32000, want: true},
		{x: -32000, y: 120, want: true},
		{x: -1910, y: 80, want: false},
		{x: 40, y: 40, want: false},
	}

	for _, tc := range tests {
		if got := sessionWindowPositionLooksHidden(tc.x, tc.y); got != tc.want {
			t.Fatalf("sessionWindowPositionLooksHidden(%d, %d) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
	}
}
