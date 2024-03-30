package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
)

type CmdLine struct {
	pid int
	cmdline string
	err error
}


func readCmdLine(pid int) (string, error) {
	path := fmt.Sprintf("/proc/%d/cmdline", pid)
	contents, err := os.ReadFile(path)
	if err == nil {
		s := string(bytes.Trim(contents, "\x00"))
		replacer := strings.NewReplacer("\x00", " ")
		s = replacer.Replace(s)
		if len(s) == 0 {
			return "", errors.New(fmt.Sprintf("Read zero size string from  %s: %s", path, s))
		} else {
			return s, nil
		}
	} else {
		return "", err
	}
}

func cmdLineReader(pid int, output chan CmdLine) {
	cmdline, err := readCmdLine(pid)
	output <- CmdLine{pid, cmdline, err}
}

func reduceCmdLines(cmdLineChannelMap map[int](chan CmdLine)) map[int]string {
	pidCmdLineMap := map[int]string{}
	for _, ch := range cmdLineChannelMap {
		for cmdline := range ch {
			if cmdline.err == nil {
				pidCmdLineMap[cmdline.pid] = cmdline.cmdline
			}
			close(ch)
		}
	}
	return pidCmdLineMap
}

func dispatchCmdLineReaders(pids []int) map[int](chan CmdLine) {
	cmdLineChannelMap := map[int](chan CmdLine){}
	for _, pid := range pids {
		chCmdLine := make(chan CmdLine, 1)
		cmdLineChannelMap[pid] = chCmdLine
		go cmdLineReader(pid, chCmdLine)
	}
	return cmdLineChannelMap
}
