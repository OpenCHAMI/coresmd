package hostname

import "testing"

func TestExpandHostnamePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		nid     int64
		id      string
		want    string
	}{
		{
			name:    "simple_nid_4_digits_zero_padded",
			pattern: "nid{04d}",
			nid:     1,
			id:      "",
			want:    "nid0001",
		},
		{
			name:    "simple_nid_2_digits_zero_padded",
			pattern: "dev-s{02d}",
			nid:     5,
			id:      "",
			want:    "dev-s05",
		},
		{
			name:    "simple_nid_3_digits_zero_padded",
			pattern: "bmc{03d}",
			nid:     42,
			id:      "",
			want:    "bmc042",
		},
		{
			name:    "id_only_pattern",
			pattern: "{id}",
			nid:     0,
			id:      "x3000c0s0b1",
			want:    "x3000c0s0b1",
		},
		{
			name:    "id_embedded_in_pattern",
			pattern: "node-{id}-svc",
			nid:     0,
			id:      "x3000c0s0b1",
			want:    "node-x3000c0s0b1-svc",
		},
		{
			name:    "nid_and_id_mixed",
			pattern: "nid{03d}-{id}",
			nid:     7,
			id:      "x1000c0s0b0",
			want:    "nid007-x1000c0s0b0",
		},
		{
			name:    "multiple_nid_patterns_with_same_value",
			pattern: "rack{02d}-node{03d}",
			nid:     7,
			id:      "",
			want:    "rack07-node007",
		},
		{
			name:    "pattern_without_any_placeholders",
			pattern: "static-hostname",
			nid:     123,
			id:      "ignored",
			want:    "static-hostname",
		},
		{
			name:    "numeric_pattern_without_leading_zero_in_format",
			pattern: "node{4d}",
			nid:     7,
			id:      "",
			want:    "node0007",
		},
		{
			name:    "large_nid_with_smaller_width_truncation_not_expected",
			pattern: "nid{02d}",
			nid:     123,
			id:      "",
			// fmt with %0*d will not truncate, just print width>=2;
			// so we expect "123"
			want: "nid123",
		},
		{
			name:    "zero_nid_with_padding",
			pattern: "nid{03d}",
			nid:     0,
			id:      "",
			want:    "nid000",
		},
		{
			name:    "negative_nid_with_padding",
			pattern: "nid{04d}",
			nid:     -1,
			id:      "",
			// fmt keeps the sign, width includes '-'
			// width 4 => "-001"
			want: "nid-001",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandHostnamePattern(tt.pattern, tt.nid, tt.id)
			if got != tt.want {
				t.Errorf("ExpandHostnamePattern(%q, %d, %q) = %q, want %q",
					tt.pattern, tt.nid, tt.id, got, tt.want)
			}
		})
	}
}
