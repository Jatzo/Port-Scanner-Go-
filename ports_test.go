package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestPortListsAreReadCorrectly(t *testing.T) {
	cases := []struct {
		name string
		spec string
		want []int
	}{
		{"a single port is read as one port", "80", []int{80}},
		{"a list is read in order", "80,443", []int{80, 443}},
		{"a range includes both ends", "8000-8003", []int{8000, 8001, 8002, 8003}},
		{"lists and ranges can be mixed", "22,80-82", []int{22, 80, 81, 82}},
		{"the first and last valid ports are accepted", "1,65535", []int{1, 65535}},
		{"a port written twice is scanned once", "80,80", []int{80}},
		{"overlapping ranges are scanned once", "80-82,81-83", []int{80, 81, 82, 83}},
		{"spaces around entries are ignored", " 80 , 443 ", []int{80, 443}},
		{"the result is sorted whatever the input order", "443,80", []int{80, 443}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParsePorts(c.spec)
			if err != nil {
				t.Fatalf("ParsePorts(%q) gave an error: %v", c.spec, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ParsePorts(%q) = %v, want %v", c.spec, got, c.want)
			}
		})
	}
}

func TestPortListsThatCannotBeReadAreRejectedWithAReason(t *testing.T) {
	cases := []struct {
		name        string
		spec        string
		wantMessage string
	}{
		{"an empty list is rejected", "", "empty"},
		{"a list of spaces is rejected", "   ", "empty"},
		{"port zero is rejected", "0", "out of range"},
		{"a port above 65535 is rejected", "65536", "out of range"},
		{"a range that runs backwards is rejected", "90-80", "backwards"},
		{"a word is rejected", "http", "not a port number"},
		{"a trailing comma is rejected", "80,", "empty entry"},
		{"a negative port is rejected", "-5", "not a port number"},
		{"a range with a missing end is rejected", "80-", "not a port number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParsePorts(c.spec)
			if err == nil {
				t.Fatalf("ParsePorts(%q) = %v, want an error", c.spec, got)
			}
			if !strings.Contains(err.Error(), c.wantMessage) {
				t.Errorf("ParsePorts(%q) error = %q, want it to mention %q", c.spec, err, c.wantMessage)
			}
		})
	}
}
