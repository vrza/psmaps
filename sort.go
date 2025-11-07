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

func sortRollups(rollups []SmemRollup, pidOwnersMap map[int]PidOwner, cmdlineMap map[int]string, key string, reverseOrder bool) []SmemRollup {
	keyLower := strings.ToLower(key)

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
