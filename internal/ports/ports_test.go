package ports

import (
	"context"
	"testing"
)

func TestReachableReportsClosedPort(t *testing.T) {
	if Reachable(context.Background(), 9) {
		t.Fatal("expected discard port to be unreachable")
	}
}

func TestUniqueSortedPorts(t *testing.T) {
	got := uniqueSortedPorts([]int{5173, 0, 2232, 5173, 70000, 3000})
	want := []int{2232, 3000, 5173}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
