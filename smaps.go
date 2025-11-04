package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type SmemHeader struct {
	start uint64
	end   uint64
}

type SmemStat struct {
	name  string
	value int
}

type SmemRollupRaw struct {
	pid      int
	contents string
	err      error
}

type SmemRollup struct {
	pid    int
	header SmemHeader
	stats  map[string]int
}

func readSmapsRollup(pid int) (string, error) {
	path := fmt.Sprintf("/proc/%d/smaps_rollup", pid)
	contents, err := os.ReadFile(path)
	if err == nil {
		s := string(contents)
		if len(s) == 0 {
			return "", fmt.Errorf("read zero size string from  %s: %s", path, s)
		} else {
			return s, nil
		}
	} else {
		// Print errors, excluding expected errors:
		// - no maps for kernel threads ("no such process")
		// - permissions (run as superuser to get stats for all processes)
		if !strings.HasSuffix(err.Error(), "no such process") &&
			!strings.HasSuffix(err.Error(), "permission denied") &&
			!strings.HasSuffix(err.Error(), "no such file or directory") {
			fmt.Printf("Error reading %s: %v\n", path, err)
		}
		return "", fmt.Errorf("PID %d is a kernel thread", pid)
	}
}

func smapsRollupReader(pid int, output chan SmemRollupRaw) {
	contents, err := readSmapsRollup(pid)
	output <- SmemRollupRaw{pid, contents, err}
}

func parseSmapsRollup(pid int, contents string) SmemRollup {
	lines := strings.Split(contents, "\n")
	header := SmemHeader{0, 0}
	stats := make(map[string]int)
	for i, line := range lines {
		if i == 0 { // header
			header = parseHeaderLine(line)
		} else { // key-value pairs
			if len(line) == 0 {
				//fmt.Printf("smapsRollupParser skipping empty stat line\n")
				continue
			}
			stat, err := parseStatLine(line)
			if err == nil {
				stats[strings.ToLower(stat.name)] = stat.value
				//final = fmt.Sprintf("%s => %d", stat.name, stat.value)
				//fmt.Printf("smapsRollupParser found stat: %s: %d\n", stat.name, stat.value)
			}
		}
	}
	//final = fmt.Sprintf("%d - %d", header.start, header.end)
	return SmemRollup{pid, header, stats}
}

func smapsRollupParser(pid int, input chan SmemRollupRaw, output chan SmemRollup) {
	//fmt.Printf("smapsRollupParser for PID %d reading\n", pid)
	contents := <-input
	//fmt.Printf("smapsRollupParser for PID %d read %d bytes\n", pid, len(contents))
	if contents.err != nil || len(contents.contents) == 0 {
		output <- SmemRollup{pid, SmemHeader{0, 0}, nil}
		return
	}
	output <- parseSmapsRollup(contents.pid, contents.contents)
}

func parseHeaderLine(headerLine string) SmemHeader {
	//fmt.Printf("parseHeaderLine: %s\n", headerLine)
	addressParts := strings.Split(headerLine, " ")
	rangeParts := strings.Split(addressParts[0], "-")
	//fmt.Printf("parseHeaderLine first part is: %s\n", rangeParts[0])

	start, err := strconv.ParseUint(rangeParts[0], 16, 64)
	if err != nil {
		log.Fatal(err)
	}
	end, err := strconv.ParseUint(rangeParts[1], 16, 64)
	if err != nil {
		log.Fatal(err)
	}
	return SmemHeader{start, end}
}

func parseStatLine(statLine string) (SmemStat, error) {
	//fmt.Printf("parsing stat line: %s\n", statLine)
	if len(statLine) == 0 {
		return SmemStat{"", 0}, errors.New("empty stat line")
	}
	statParts := strings.Split(statLine, ":")
	key := statParts[0]
	valueString := statParts[1]
	valueParts := strings.Split(strings.TrimSpace(valueString), " ")
	value, err := strconv.Atoi(valueParts[0])
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Printf("parseStatLine: %s => %d\n", key, value)
	return SmemStat{key, value}, nil
}

// dispatches smem_rollup file parser goroutines:
// - one file reader goroutine per pid
// - one parser goroutine per pid
func dispatchSmemRollupParsers(pids []int) map[int](chan SmemRollup) {
	pidSmemRollupParserChannelMap := map[int](chan SmemRollup){}
	for _, pid := range pids {
		chSmemRollupReaderOutput := make(chan SmemRollupRaw, 1)
		go smapsRollupReader(pid, chSmemRollupReaderOutput)

		chSmemRollupParserOutput := make(chan SmemRollup, 1)
		pidSmemRollupParserChannelMap[pid] = chSmemRollupParserOutput
		go smapsRollupParser(pid, chSmemRollupReaderOutput, chSmemRollupParserOutput)
	}
	return pidSmemRollupParserChannelMap
}

// iterative reducer
// iterates over channels and waits for them
func reduceSmemRollupParsers(pidSmemRollupParserChannelMap map[int](chan SmemRollup)) []SmemRollup {
	var rollupSlice []SmemRollup
	for _, ch := range pidSmemRollupParserChannelMap {
		for rollup := range ch {
			if len(rollup.stats) > 0 {
				rollupSlice = append(rollupSlice, rollup)
			}
			close(ch)
		}
	}
	return rollupSlice
}

// reflect.select reducer
// selects a channel that has data using reflect.SelectCase
// because of the overhead of reflect.SelectCase, in this use case it's not really faster
func reduceSmemRollupParsersSelect(pidSmemRollupParserChannelMap map[int](chan SmemRollup)) []SmemRollup {
	numCases := len(pidSmemRollupParserChannelMap)
	pids := make([]int, numCases)
	i := 0
	for pid := range pidSmemRollupParserChannelMap {
		pids[i] = pid
		i++
	}

	parserCases := make([]reflect.SelectCase, numCases)

	for i := range pids {
		ch := pidSmemRollupParserChannelMap[pids[i]]
		parserCases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}

	var rollupSlice []SmemRollup

	remainingParsers := len(parserCases)
	for remainingParsers > 0 {
		chosen, recv, ok := reflect.Select(parserCases)
		if !ok {
			fmt.Printf("reduceSmemRollupParsersSelect: Selected channel %d has been closed, zeroing out the channel to disable the case\n", chosen)
			parserCases[chosen].Chan = reflect.ValueOf(nil)
			continue
		}

		rollup := recv.Interface().(SmemRollup)
		//fmt.Printf("Read from channel %d %#v and received %v\n", chosen, pidSmemRollupParserChannelMap[pids[chosen]], rollup)

		remainingParsers -= 1
		close(pidSmemRollupParserChannelMap[pids[chosen]])
		parserCases[chosen].Chan = reflect.ValueOf(nil)

		if len(rollup.stats) > 0 {
			rollupSlice = append(rollupSlice, rollup)
			//fmt.Printf("PID %d: %s\n", pids[chosen], parsedLine)
		}
	}
	return rollupSlice
}
