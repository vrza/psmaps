package main

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/crypto/ssh/terminal"
	"log"
	"os"
	"strconv"
	"unicode/utf8"
)

// processes returns the list of all process IDs
func allProcesses() []int {
	files, err := os.ReadDir("/proc/")
	if err != nil {
		log.Fatal(err)
	}
	var processes []int
	for _, file := range files {
		if pid, err := strconv.Atoi(file.Name()); err == nil {
			processes = append(processes, pid)
		}
	}
	return processes
}

// calculate width of columns other than command line
func otherColumnsWidth(rollups map[int]SmemRollup, pidOwnersMap map[int]PidOwner) int {
	spacingWidth := 11
	pidWidth := 0
	userWidth := 0
	ussWidth := 0
	pssWidth := 0
	rssWidth := 0

	for pid, rollup := range rollups {
		l := len(fmt.Sprintf("%d", pid))
		if l > pidWidth {
			pidWidth = l
		}

		user := pidOwnersMap[pid].username
		if user == "" {
			user = strconv.Itoa(pidOwnersMap[pid].uid)
		}
		l = len(fmt.Sprintf("%s", user))
		if l > userWidth {
			userWidth = l
		}

		uss := rollup.stats["Pss_Clean"] + rollup.stats["Pss_Dirty"]
		l = len(fmt.Sprintf("%d", uss))
		if l > ussWidth {
			ussWidth = l
		}

		pss := rollup.stats["Pss"]
		l = len(fmt.Sprintf("%d", pss))
		if l > pssWidth {
			pssWidth = l
		}

		rss := rollup.stats["Rss"]
		l = len(fmt.Sprintf("%d", rss))
		if l > rssWidth {
			rssWidth = l
		}
	}

	return spacingWidth + pidWidth + userWidth + ussWidth + pssWidth + rssWidth
}

// render output table to stdout
func render(rollups map[int]SmemRollup, pidOwnersMap map[int]PidOwner, cmdlineMap map[int]string) {
	terminalWidth, _, _ := terminal.GetSize(int(os.Stdout.Fd()))
	cmdWidth := terminalWidth - otherColumnsWidth(rollups, pidOwnersMap)

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
	for pid, rollup := range rollups {
		uss := rollup.stats["Pss_Clean"] + rollup.stats["Pss_Dirty"]
		pss := rollup.stats["Pss"]
		rss := rollup.stats["Rss"]
		user := pidOwnersMap[pid].username
		if user == "" {
			user = strconv.Itoa(pidOwnersMap[pid].uid)
		}
		command := cmdlineMap[pid]
		if utf8.RuneCountInString(command) > cmdWidth {
			command = string([]rune(cmdlineMap[pid])[0:cmdWidth])
		}
		t.AppendRow(table.Row{pid, user, uss, pss, rss, command})
	}

	t.Render()
}

func main() {
	//trace.Start(os.Stderr)
	//defer trace.Stop()

	pids := []int{}
	if len(os.Args) > 1 {
		for i := 1; i < len(os.Args); i++ {
			pid, err := strconv.Atoi(os.Args[i])
			if err == nil {
				pids = append(pids, pid)
			}
		}
	} else {
		pids = allProcesses()
	}

	// dispatch
	pidSmemRollupParserChannelMap := dispatchSmemRollupParsers(pids)
	pidOwnerChannelMap := dispatchPidOwners(pids)
	comdlineChannelMap := dispatchCmdLineReaders(pids)

	// collect

	//rollups := reduceSmemRollupParsers(pidSmemRollupParserChannelMap)
	rollups := reduceSmemRollupParsersSelect(pidSmemRollupParserChannelMap)

	pidOwnersMap := reducePidOwners(pidOwnerChannelMap)
	//pidOwnersMap := reducePidOwnersSelect(pidOwnerChannelMap)

	cmdlineMap := reduceCmdLines(comdlineChannelMap)

	render(rollups, pidOwnersMap, cmdlineMap)
}
