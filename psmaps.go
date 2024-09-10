package main

import (
	"flag"
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
	pidWidth := 3
	userWidth := 4
	ussWidth := 3
	pssWidth := 3
	rssWidth := 3

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

func terminalWidth() int {
	width, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, _, err = terminal.GetSize(int(os.Stdin.Fd()))
		if err != nil {
			width = 80
		}
	}
	return width
}

// render output table to stdout
func render(rollups map[int]SmemRollup, pidOwnersMap map[int]PidOwner, cmdlineMap map[int]string, isWideOutput bool) {
	cmdWidth := terminalWidth() - otherColumnsWidth(rollups, pidOwnersMap)
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
	for pid, rollup := range rollups {
		uss := rollup.stats["Pss_Clean"] + rollup.stats["Pss_Dirty"]
		pss := rollup.stats["Pss"]
		rss := rollup.stats["Rss"]
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

const flagHelpDescription = "print help information"
const flagWideDescription = "always print full command line"

func printUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [OPTION]... [PID]...\n", os.Args[0])
	fmt.Fprintf(flag.CommandLine.Output(), `Options:
  -h, --help   %s
  -w, --wide   %s
`, flagHelpDescription, flagWideDescription)
}

func main() {
	//trace.Start(os.Stderr)
	//defer trace.Stop()

	// parse command line arguments
	var help, isWideOutput bool
	flag.BoolVar(&help, "help", false, flagHelpDescription)
	flag.BoolVar(&help, "h", false, flagHelpDescription)
	flag.BoolVar(&isWideOutput, "wide", false, flagWideDescription)
	flag.BoolVar(&isWideOutput, "w", false, flagWideDescription)
	flag.Usage = printUsage
	flag.Parse()

	if help {
		printUsage()
		os.Exit(0)
	}

	pids := []int{}
	args := flag.Args()
	if len(args) > 0 {
		for i := 0; i < len(args); i++ {
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

	render(rollups, pidOwnersMap, cmdlineMap, isWideOutput)
}
