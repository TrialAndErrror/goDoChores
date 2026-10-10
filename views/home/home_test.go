package home

import (
	"math/rand"
	"testing"
	"time"
)

func items(n int) []CombinedItem {
	out := make([]CombinedItem, n)
	for i := range out {
		out[i] = CombinedItem{ID: uint(i + 1), Name: "item"}
	}
	return out
}

func TestPageItems(t *testing.T) {
	cases := []struct {
		name         string
		total        int
		offset       int
		wantLen      int
		wantNext     int
		wantHasMore  bool
		wantFirstIdx int // index into the source slice of the first returned item
	}{
		{"empty", 0, 0, 0, 0, false, 0},
		{"fewer than first batch", 5, 0, 5, 5, false, 0},
		{"exactly first batch", 12, 0, 12, 12, false, 0},
		{"first batch with more", 30, 0, 12, 12, true, 0},
		{"second batch", 30, 12, 6, 18, true, 12},
		{"last full batch", 30, 24, 6, 30, false, 24},
		{"partial last batch", 20, 18, 2, 20, false, 18},
		{"offset at end", 12, 12, 0, 12, false, 0},
		{"offset past end", 12, 99, 0, 12, false, 0},
		{"negative offset is the first page", 30, -4, 12, 12, true, 0},
	}
	for _, c := range cases {
		src := items(c.total)
		page, next, more := pageItems(src, c.offset)
		if len(page) != c.wantLen || next != c.wantNext || more != c.wantHasMore {
			t.Errorf("%s: got (len %d, next %d, more %v), want (len %d, next %d, more %v)",
				c.name, len(page), next, more, c.wantLen, c.wantNext, c.wantHasMore)
			continue
		}
		if c.wantLen > 0 && page[0].ID != src[c.wantFirstIdx].ID {
			t.Errorf("%s: first item is %v, want %v", c.name, page[0].ID, src[c.wantFirstIdx].ID)
		}
	}
}

// Walking the pages must visit every item exactly once, in order.
func TestPageItemsCoversEverythingOnce(t *testing.T) {
	for _, total := range []int{0, 1, 11, 12, 13, 17, 18, 19, 30, 100} {
		src := items(total)
		var seen []uint
		offset := 0
		for steps := 0; ; steps++ {
			if steps > total+2 {
				t.Fatalf("total %d: paging did not terminate", total)
			}
			page, next, more := pageItems(src, offset)
			for _, it := range page {
				seen = append(seen, it.ID.(uint))
			}
			if !more {
				break
			}
			offset = next
		}
		if len(seen) != total {
			t.Errorf("total %d: saw %d items", total, len(seen))
			continue
		}
		for i, id := range seen {
			if id != uint(i+1) {
				t.Errorf("total %d: position %d has id %d", total, i, id)
				break
			}
		}
	}
}

func TestIDNumber(t *testing.T) {
	if idNumber(uint(7)) != 7 || idNumber(int(9)) != 9 || idNumber("x") != 0 || idNumber(nil) != 0 {
		t.Error("idNumber returned an unexpected value")
	}
}

func TestSortItemsIsDeterministic(t *testing.T) {
	d1 := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := d1.AddDate(0, 0, 1)
	want := []CombinedItem{
		{ID: int(3), Type: "chore", Date: d1},
		{ID: int(9), Type: "chore", Date: d1},
		{ID: uint(2), Type: "task", Date: d1},
		{ID: uint(5), Type: "task", Date: d1},
		{ID: int(1), Type: "chore", Date: d2},
		{ID: uint(1), Type: "task", Date: d2},
	}
	for seed := int64(0); seed < 50; seed++ {
		got := append([]CombinedItem(nil), want...)
		rand.New(rand.NewSource(seed)).Shuffle(len(got), func(i, j int) { got[i], got[j] = got[j], got[i] })
		sortItems(got)
		for i := range want {
			if got[i].ID != want[i].ID || got[i].Type != want[i].Type {
				t.Fatalf("seed %d: position %d is %s/%v, want %s/%v", seed, i, got[i].Type, got[i].ID, want[i].Type, want[i].ID)
			}
		}
	}
}
