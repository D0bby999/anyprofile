// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"testing"
)

// Reproducible fixtures: one account and one extension per profile, separate
// temporary databases. Run with -run '^$' -bench BenchmarkProfileList -benchmem.
func BenchmarkProfileList(b *testing.B) {
	for _, n := range []int{100, 300, 500} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := seedPageTest(b, n)
			for _, tc := range []struct {
				name   string
				filter ProfileFilter
				count  bool
			}{
				{name: "all"},
				{name: "page50", filter: ProfileFilter{Limit: 50}},
				{name: "search", filter: ProfileFilter{Query: "kenh"}},
				{name: "page50_with_count", filter: ProfileFilter{Limit: 50}, count: true},
			} {
				b.Run(tc.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := s.ListProfiles(tc.filter); err != nil {
							b.Fatal(err)
						}
						if tc.count {
							if _, err := s.CountProfiles(tc.filter); err != nil {
								b.Fatal(err)
							}
						}
					}
				})
			}
		})
	}
}
