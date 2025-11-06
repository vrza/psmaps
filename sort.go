package main

import (
	"cmp"
	"slices"
	"strings"
)

// Wraps an anoynymous getter function and produces a comparator
// suitable for slices.SortFunc
func makeComparator[T cmp.Ordered](getter func(SmemRollup) T) func(a, b SmemRollup) int {
	return func(a, b SmemRollup) int {
		return cmp.Compare(getter(a), getter(b))
	}
}

// Sorts rollups based on string keys: each key maps to a lambda that gets
// the values to pass to cmp.Compare
func sortRollups(rollups []SmemRollup, pidOwnersMap map[int]PidOwner, cmdlineMap map[int]string, key string, reverseOrder bool) []SmemRollup {
	keyLower := strings.ToLower(key)

	comparators := map[string]func(a, b SmemRollup) int{
		"pid": makeComparator(func(r SmemRollup) int { return r.pid }),

		"uss": makeComparator(func(r SmemRollup) int { return r.getUSS() }),
		"pss": makeComparator(func(r SmemRollup) int { return r.getPSS() }),
		"rss": makeComparator(func(r SmemRollup) int { return r.getRSS() }),

		"user": makeComparator(func(r SmemRollup) string {
			return pidOwnersMap[r.pid].username
		}),

		"command": makeComparator(func(r SmemRollup) string {
			return cmdlineMap[r.pid]
		}),
	}

	comparator := comparators[keyLower] // key is validated upstream

	slices.SortFunc(rollups, func(a, b SmemRollup) int {
		c := comparator(a, b)
		if reverseOrder {
			c *= -1
		}
		return c
	})

	return rollups
}
