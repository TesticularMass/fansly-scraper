package ui

import (
	"reflect"
	"testing"
)

func TestParseIndividualPostLinks(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			"single id",
			"837364635748286464",
			[]string{"837364635748286464"},
		},
		{
			"url with query",
			"https://fansly.com/post/837364635748286464?ngsw-bypass=true",
			[]string{"837364635748286464"},
		},
		{
			"mixed separators",
			"111, 222 333;444\n555",
			[]string{"111", "222", "333", "444", "555"},
		},
		{
			"non-numeric tokens dropped",
			"abc 123 https://fansly.com/post/notanid",
			[]string{"123"},
		},
		{
			"empty input",
			"   ",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIndividualPostLinks(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIndividualPostLinks(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
