package control

import "testing"

func TestTaskWarrantsPlanner(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"   ", false},
		{"/init", false},
		{"what does this function do?", false}, // low-risk question �?executor only
		{"why did the test fail", false},
		{"解释一下这段代�?, false},
		{"fix the bug", true},        // terse, but a work request �?still planned
		{"add a login button", true}, // ditto
		{"implement the new caching layer across the backend", true},
	}
	for _, c := range cases {
		if got := TaskWarrantsPlanner(c.input); got != c.want {
			t.Errorf("TaskWarrantsPlanner(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}
