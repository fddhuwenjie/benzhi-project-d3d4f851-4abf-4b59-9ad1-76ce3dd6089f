package main

import "testing"

func TestAddressFrom(t *testing.T) {
	tests := []struct {
		name, env, value string
		set, ok          bool
	}{
		{"default", "", defaultAddress, false, true},
		{"port env", "19123", defaultAddress, false, true},
		{"explicit wins", "19123", "127.0.0.1:19234", true, true},
		{"missing host", "", ":19081", true, false},
		{"wildcard", "", "0.0.0.0:19081", true, false},
		{"low port", "", "127.0.0.1:80", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := addressFrom(tc.env, tc.value, tc.set)
			if (err == nil) != tc.ok {
				t.Fatalf("结果不符: %v", err)
			}
		})
	}
}
