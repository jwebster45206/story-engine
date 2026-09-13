package llm

import "testing"

func TestPickCompleteModel(t *testing.T) {
	tests := []struct {
		adj, backend, primary, want string
	}{
		{"adj", "back", "prim", "adj"},
		{"", "back", "prim", "back"},
		{"", "", "prim", "prim"},
		{"  ", "back", "prim", "back"},
	}
	for _, tt := range tests {
		if got := pickCompleteModel(tt.adj, tt.backend, tt.primary); got != tt.want {
			t.Errorf("pickCompleteModel(%q,%q,%q) = %q, want %q", tt.adj, tt.backend, tt.primary, got, tt.want)
		}
	}
}
