package main

import (
	"fmt"
	"os"
	"strconv"
	"unicode/utf8"

	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/term"
)

// render size in kilobytes to a string, optionally human-readable
func kiloBytesToString(value int, humanReadable bool) string {
	if humanReadable {
		return humanize.IBytes(uint64(value * 1024))
	}
	return fmt.Sprintf("%d", value)
}

// if output is to a terminal, get its width
func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, _, err = term.GetSize(int(os.Stdin.Fd()))
		if err != nil {
			width = 80
		}
	}
	return width
}

// calculate width of columns other than command line
func otherColumnsWidth(rollups []SmemRollup, pidOwnersMap map[int]PidOwner, humanReadable bool) int {
	spacingWidth := 11
	pidWidth := 3
	userWidth := 4
	ussWidth := 3
	pssWidth := 3
	rssWidth := 3

	for _, rollup := range rollups {
		l := len(fmt.Sprintf("%d", rollup.pid))
		if l > pidWidth {
			pidWidth = l
		}

		user := pidOwnersMap[rollup.pid].username
		if user == "" {
			user = strconv.Itoa(pidOwnersMap[rollup.pid].uid)
		}
		l = len(user)
		if l > userWidth {
			userWidth = l
		}

		uss := rollup.stats[StatPrivateClean] + rollup.stats[StatPrivateDirty]
		l = len(kiloBytesToString(uss, humanReadable))
		if l > ussWidth {
			ussWidth = l
		}

		pss := rollup.stats[StatPSS]
		l = len(kiloBytesToString(pss, humanReadable))
		if l > pssWidth {
			pssWidth = l
		}

		rss := rollup.stats[StatRSS]
		l = len(kiloBytesToString(rss, humanReadable))
		if l > rssWidth {
			rssWidth = l
		}
	}

	return spacingWidth + pidWidth + userWidth + ussWidth + pssWidth + rssWidth
}

// render output table to stdout
func render(rollups []SmemRollup, pidOwnersMap map[int]PidOwner, cmdlineMap map[int]string, isWideOutput bool, humanReadable bool) {
	cmdWidth := terminalWidth() - otherColumnsWidth(rollups, pidOwnersMap, humanReadable)
	if cmdWidth < 7 {
		cmdWidth = 7
		isWideOutput = true
	}

	t := table.NewWriter()

	t.SetOutputMirror(os.Stdout)
	t.SuppressTrailingSpaces()
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight, AlignFooter: text.AlignRight, AlignHeader: text.AlignRight},
		{Number: 2, Align: text.AlignLeft, AlignFooter: text.AlignLeft, AlignHeader: text.AlignLeft},
		{Number: 3, Align: text.AlignRight, AlignFooter: text.AlignRight, AlignHeader: text.AlignRight},
		{Number: 4, Align: text.AlignRight, AlignFooter: text.AlignRight, AlignHeader: text.AlignRight},
		{Number: 5, Align: text.AlignRight, AlignFooter: text.AlignRight, AlignHeader: text.AlignRight},
		{Number: 6, Align: text.AlignLeft, AlignFooter: text.AlignLeft, AlignHeader: text.AlignLeft},
	})
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateHeader = false
	t.Style().Options.SeparateColumns = false
	t.Style().Options.SeparateRows = false

	t.AppendHeader(table.Row{"PID", "User", "USS", "PSS", "RSS", "Command"})
	for _, rollup := range rollups {
		pid := rollup.pid
		uss := kiloBytesToString((rollup.stats[StatPrivateClean] + rollup.stats[StatPrivateDirty]), humanReadable)
		pss := kiloBytesToString((rollup.stats[StatPSS]), humanReadable)
		rss := kiloBytesToString(rollup.stats[StatRSS], humanReadable)
		user := pidOwnersMap[pid].username
		if user == "" {
			user = strconv.Itoa(pidOwnersMap[pid].uid)
		}
		command := cmdlineMap[pid]
		if !isWideOutput && utf8.RuneCountInString(command) > cmdWidth {
			command = string([]rune(cmdlineMap[pid])[0:cmdWidth])
		}
		t.AppendRow(table.Row{pid, user, uss, pss, rss, command})
	}

	t.Render()
}
