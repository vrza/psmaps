package main

import (
	"cmp"
	"slices"
	"strings"
)

// Wraps an anoynymous getter function and produces a comparator
// suitable for slices.SortFunc
func makeComparator[T cmp.Ordered](getter func(SmemRollup) T) func(SmemRollup, SmemRollup) int {
	return func(a, b SmemRollup) int {
		return cmp.Compare(getter(a), getter(b))
	}
}

// Sorts rollups based on string keys: each key maps to a lambda that gets
// the values to pass to cmp.Compare
func sortRollups(rollups []SmemRollup, pidOwnersMap map[int]PidOwner, cmdlineMap map[int]string, key string, reverseOrder bool) []SmemRollup {
	keyLower := strings.ToLower(key)

	comparators := map[string]func(SmemRollup, SmemRollup) int{
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
