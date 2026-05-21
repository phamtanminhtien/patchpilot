package slow

import (
	"testing"
	"time"
)

func TestSlow(t *testing.T) {
	time.Sleep(30 * time.Second)
}
