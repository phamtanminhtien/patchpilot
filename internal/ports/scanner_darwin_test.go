//go:build darwin

package ports

import "testing"

func TestParseLsofPorts(t *testing.T) {
	got := uniqueSortedPorts(parseLsofPorts("p123\nnTCP *:2232 (LISTEN)\nnTCP 127.0.0.1:5173 (LISTEN)\n"))
	want := []int{2232, 5173}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
