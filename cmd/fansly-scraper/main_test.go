package main

import (
	"reflect"
	"testing"
)

func TestParseUsernames(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"user1", []string{"user1"}},
		{"user1,user2,user3", []string{"user1", "user2", "user3"}},
		{" user1 ,  user2 ,user3 ", []string{"user1", "user2", "user3"}},
		{"", nil},
		{",,,", nil},
	}

	for _, tc := range tests {
		got := parseUsernames(tc.input)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("parseUsernames(%q) = %v; want %v", tc.input, got, tc.want)
		}
	}
}
