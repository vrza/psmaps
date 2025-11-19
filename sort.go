package main

import (
	"cmp"
	"slices"
	"strings"
)

type RollupGetter[T cmp.Ordered] func(SmemRollup) T
type RollupComparator func(SmemRollup, SmemRollup) int

func makeComparator[T cmp.Ordered](getter RollupGetter[T]) RollupComparator {
	return func(a, b SmemRollup) int {
		return cmp.Compare(getter(a), getter(b))
	}
}

// Sorts rollups by one of the supported keys.
// Keys are validated upstream.
// Helper abstractions:
// comparator function -- takes two SmemRollups and compares them
// getter function -- used by comparator internally to obtain values to feed into cmp.Compare
// comparator factory -- takes a getter function and returns a comparator function
func sortRollups(rollups []SmemRollup, pidOwnersMap map[int]PidOwner, cmdlineMap map[int]string, key string, reverseOrder bool) []SmemRollup {
	comparators := map[string]RollupComparator{
		"pid": makeComparator(SmemRollup.PID),
		"uss": makeComparator(SmemRollup.USS),
		"pss": makeComparator(SmemRollup.PSS),
		"rss": makeComparator(SmemRollup.RSS),
		"user": makeComparator(func(r SmemRollup) string {
			return pidOwnersMap[r.PID()].username
		}),
		"command": makeComparator(func(r SmemRollup) string {
			return cmdlineMap[r.PID()]
		}),
	}

	comparator := comparators[strings.ToLower(key)]

	slices.SortFunc(rollups, func(a, b SmemRollup) int {
		c := comparator(a, b)
		if reverseOrder {
			c *= -1
		}
		return c
	})

	return rollups
}
