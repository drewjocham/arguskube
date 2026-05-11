package models

import "testing"

func TestDriftFilterOffset(t *testing.T) {
	tests := []struct {
		name   string
		filter *DriftFilter
		want   int
	}{
		{name: "default page 1", filter: &DriftFilter{Limit: 50}, want: 0},
		{name: "page 2", filter: &DriftFilter{Page: 2, Limit: 50}, want: 50},
		{name: "page 3 limit 25", filter: &DriftFilter{Page: 3, Limit: 25}, want: 50},
		{name: "defaults to page 1", filter: &DriftFilter{}, want: 0},
		{name: "defaults to limit 50", filter: &DriftFilter{Page: 2}, want: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Offset(); got != tt.want {
				t.Errorf("Offset() = %d, want %d", got, tt.want)
			}
		})
	}
}
