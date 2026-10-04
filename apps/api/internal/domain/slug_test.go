package domain

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Kingfisher Villa":    "kingfisher-villa",
		"  Coral Bay House  ": "coral-bay-house",
		"Ishq's Villa & Spa!": "ishq-s-villa-spa",
		"Villa 42":            "villa-42",
		"---":                 "",
		"An extremely long villa name that goes on and on": "an-extremely-long-villa-name-that-goes-o",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateSlug(t *testing.T) {
	valid := []string{"kingfisher", "coral-bay", "villa42"}
	for _, s := range valid {
		if err := ValidateSlug(s); err != nil {
			t.Errorf("ValidateSlug(%q) returned %v, want nil", s, err)
		}
	}
	invalid := []string{"", "ab", "-villa", "villa-", "Villa", "villa_one", "api", "www"}
	for _, s := range invalid {
		if err := ValidateSlug(s); err == nil {
			t.Errorf("ValidateSlug(%q) returned nil, want an error", s)
		}
	}
}
