//go:build darwin

package darwin

import (
	"reflect"
	"testing"
)

func TestNormalizePrintRangesSortsMergesAndDropsInvalid(t *testing.T) {
	got := normalizePrintRanges([]PrintPageRange{
		{From: 8, To: 9},
		{From: 2, To: 3},
		{From: 4, To: 6},
		{From: 6, To: 8},
		{From: 12, To: 11},
		{From: 20, To: 20},
	})
	want := []PrintPageRange{{From: 2, To: 9}, {From: 20, To: 20}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizePrintRanges() = %#v, want %#v", got, want)
	}
}

func TestPrintPageSelectedUsesInclusiveOneBasedRanges(t *testing.T) {
	ranges := []PrintPageRange{{From: 2, To: 3}, {From: 7, To: 7}}
	for page, want := range map[int]bool{
		1: false,
		2: true,
		3: true,
		4: false,
		6: false,
		7: true,
		8: false,
	} {
		if got := printPageSelected(page, ranges); got != want {
			t.Errorf("printPageSelected(%d) = %v, want %v", page, got, want)
		}
	}
}

func TestNormalizePrintRangesHandlesLargestPageNumber(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	got := normalizePrintRanges([]PrintPageRange{
		{From: maxInt, To: maxInt},
		{From: maxInt - 2, To: maxInt - 1},
	})
	want := []PrintPageRange{{From: maxInt - 2, To: maxInt}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizePrintRanges() = %#v, want %#v", got, want)
	}
}
