package entity

import (
	"encoding/json"
	"testing"
)

func TestPage(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name   string
		page   Page
		valid  bool
		offset int
	}{
		{"defaults", Page{}, true, 0},
		{"beyond 201", Page{Page: 202, PageSize: 100}, true, 20100},
		{"last safe offset", Page{Page: maxInt/100 + 1, PageSize: 100}, true, (maxInt / 100) * 100},
		{"negative page", Page{Page: -1}, false, 0},
		{"negative size", Page{PageSize: -1}, false, 0},
		{"size too large", Page{PageSize: 101}, false, 0},
		{"offset overflow", Page{Page: maxInt/100 + 2, PageSize: 100}, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.page.Normalize()
			err := tc.page.Validate()
			if (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v", err)
			}
			if tc.valid && (tc.page.Offset() != tc.offset || tc.page.Limit() != tc.page.PageSize) {
				t.Fatalf("page = %+v, offset = %d", tc.page, tc.page.Offset())
			}
		})
	}
	page := Page{}
	page.Normalize()
	if page.Page != DefaultPage || page.PageSize != DefaultPageSize {
		t.Fatalf("defaults = %+v", page)
	}
	body, err := json.Marshal(PageList[string]{Page: page, Total: 0, List: []string{}})
	if err != nil || string(body) != `{"page":1,"page_size":20,"total":0,"list":[]}` {
		t.Fatalf("JSON = %s, %v", body, err)
	}
}
