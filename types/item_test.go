package hyperfeed

import (
	"testing"
)

func TestItemRowCompliance(t *testing.T) {
	var item Item = Item{}
	if err := RowComplianceTest(&item); err != nil {
		t.Fatalf("Item Row compliance failed: %v", err)
	}
}
