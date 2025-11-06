/*
psmaps reports memory usage of Linux processes, including USS, PSS, and RSS.

The memory metrics are defined as follows:

- USS - unique set size, the amount of memory unique to a process, i.e. not shared with any other process.

- PSS - proportional set size, the process’s unshared memory plus a proportional share of memory shared with other processes.

- RSS - resident set size, the total memory resident in RAM for a process, including all private pages and all shared pages.

All values represent memory resident in RAM (not swapped).

Values are shown in KiB by default.

Usage:

psmaps [flags] [pid ...]

Flags:

	--help
		Print help information.

	-w, --wide
		Always print the full command line, even if it exceeds the screen width.

	-k, --key
		Select field to sort output on.

	-r, --reverse
		Sort in reverse order.

	-h, --human-readable
		Print sizes in human readable format (e.g. MiB, GiB).
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

const procDir = "/proc"

// returns the list of PIDs of all processes
func allProcesses() []int {
	files, err := os.ReadDir(procDir + "/")
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

const (
	ExitSuccess          = 0
	ExitInvalidArguments = 1
)

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

	// validate sort key
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

	// select PIDs
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

	// dispatch goroutines
	pidSmemRollupParserChannelMap := dispatchSmemRollupParsers(pids)
	pidOwnerChannelMap := dispatchPidOwners(pids)
	comdlineChannelMap := dispatchCmdLineReaders(pids)

	// collect results

	//rollups := reduceSmemRollupParsers(pidSmemRollupParserChannelMap)
	rollups := reduceSmemRollupParsersSelect(pidSmemRollupParserChannelMap)

	pidOwnersMap := reducePidOwners(pidOwnerChannelMap)
	//pidOwnersMap := reducePidOwnersSelect(pidOwnerChannelMap)

	cmdlineMap := reduceCmdLines(comdlineChannelMap)

	// sort
	sortRollups(rollups, pidOwnersMap, cmdlineMap, sortKey, reverseOrder)

	// output
	render(rollups, pidOwnersMap, cmdlineMap, wideOutput, humanReadable)
}
