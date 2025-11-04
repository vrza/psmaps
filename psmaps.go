package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/term"
)

const (
	ExitSuccess          = 0
	ExitInvalidArguments = 1
)

const (
	StatRSS          = "rss"
	StatPSS          = "pss"
	StatPrivateClean = "private_clean"
	StatPrivateDirty = "private_dirty"
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

// render size in kilobytes to a string, optionally human-readable
func kiloBytesToString(value int, humanReadable bool) string {
	if humanReadable {
		return humanize.IBytes(uint64(value * 1024))
	}
	return fmt.Sprintf("%d", value)
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

func sortRollups(rollups []SmemRollup, pidOwnersMap map[int]PidOwner, cmdlineMap map[int]string, key string, reverseOrder bool) []SmemRollup {
	keyLower := strings.ToLower(key)
	slices.SortFunc(rollups, func(a, b SmemRollup) int {
		var cmp int

		switch keyLower {
		case "pid": // not in stats map, integer
			cmp = a.pid - b.pid
		case "uss": // dynamically computed, integer
			ussA := a.stats[StatPrivateClean] + a.stats[StatPrivateDirty]
			ussB := b.stats[StatPrivateClean] + b.stats[StatPrivateDirty]
			cmp = ussA - ussB
		case "user": // not in stats map, string
			userA := pidOwnersMap[a.pid].username
			userB := pidOwnersMap[b.pid].username
			cmp = strings.Compare(userA, userB)
		case "command": // not in stats map, string
			cmp = strings.Compare(cmdlineMap[a.pid], cmdlineMap[b.pid])
		default: // by case-instensitive key in stats map, integer
			cmp = a.stats[keyLower] - b.stats[keyLower]
		}

		if reverseOrder {
			cmp *= -1
		}
		return cmp
	})
	return rollups
}

const flagHelpDescription = "print help information"
const flagWideDescription = "always print full command line"
const flagSortKeyDescription = "field to sort output on"
const flagReverseSortDescription = "sort in reverse order"
const flagHumanReadableDescription = "print sizes in human readable format"

func printUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [OPTION]... [PID]...\n", os.Args[0])
	fmt.Fprintf(flag.CommandLine.Output(), `Options:
  --help                %s
  -w, --wide            %s
  -k, --key             %s
  -r, --reverse         %s
  -h, --human-readable  %s
`,
		flagHelpDescription,
		flagWideDescription,
		flagSortKeyDescription,
		flagReverseSortDescription,
		flagHumanReadableDescription)
}

func main() {
	//trace.Start(os.Stderr)
	//defer trace.Stop()

	// parse command line arguments
	var help, wideOutput, reverseOrder, humanReadable bool
	var sortKey string
	flag.BoolVar(&help, "help", false, flagHelpDescription)
	flag.BoolVar(&wideOutput, "wide", false, flagWideDescription)
	flag.BoolVar(&wideOutput, "w", false, flagWideDescription)
	flag.StringVar(&sortKey, "key", "pid", flagSortKeyDescription)
	flag.StringVar(&sortKey, "k", "pid", flagSortKeyDescription)
	flag.BoolVar(&reverseOrder, "reverse", false, flagReverseSortDescription)
	flag.BoolVar(&reverseOrder, "r", false, flagReverseSortDescription)
	flag.BoolVar(&humanReadable, "human-readable", false, flagWideDescription)
	flag.BoolVar(&humanReadable, "h", false, flagWideDescription)
	flag.Usage = printUsage
	flag.Parse()

	if help {
		printUsage()
		os.Exit(ExitSuccess)
	}

	allowedSortKeys := map[string]bool{
		"pid":     true,
		"rss":     true,
		"pss":     true,
		"uss":     true,
		"user":    true,
		"command": true,
	}
	if !allowedSortKeys[strings.ToLower(sortKey)] {
		fmt.Fprintf(os.Stderr, "error: unknown sort key: %s\n", sortKey)
		os.Exit(ExitInvalidArguments)
	}

	pids := []int{}
	args := flag.Args()
	if len(args) > 0 {
		for i := range args {
			pid, err := strconv.Atoi(args[i])
			if err == nil && pid > 0 {
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

	sortRollups(rollups, pidOwnersMap, cmdlineMap, sortKey, reverseOrder)

	render(rollups, pidOwnersMap, cmdlineMap, wideOutput, humanReadable)
}
