package main

import (
	"fmt"
	"os"
	"os/user"
	"reflect"
	"strconv"
	"sync"
	"syscall"
)

type PidOwner struct {
	pid      int
	uid      int
	username string
	err      error
}

var (
	uidUsernameCache      = map[int]string{}
	uidUsernameCacheMutex = sync.RWMutex{}
)

func userFromUID(uid int) string {
	uidUsernameCacheMutex.RLock()
	cachedUser, ok := uidUsernameCache[uid]
	uidUsernameCacheMutex.RUnlock()
	if ok {
		return cachedUser
	}
	user, err := user.LookupId(strconv.Itoa(uid))
	if err == nil {
		uidUsernameCacheMutex.Lock()
		uidUsernameCache[uid] = user.Username
		uidUsernameCacheMutex.Unlock()
		return user.Username
	}
	return ""
}

func pidOwner(pid int, output chan PidOwner) {
	info, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err == nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			var uid int = int(stat.Uid)
			username := userFromUID(uid)
			//fmt.Printf("PidOwner sending %d - %d\n", pid, uid)
			output <- PidOwner{pid, uid, username, nil}
		}
	} else {
		//fmt.Printf("could not stat /proc/%d\n", pid)
		output <- PidOwner{pid, -1, "", err}
	}
}

// dispatches goroutines to find users owning pids,
// one goroutine per pid
func dispatchPidOwners(pids []int) map[int](chan PidOwner) {
	pidOwnerChannelMap := map[int](chan PidOwner){}
	for _, pid := range pids {
		chPidOwner := make(chan PidOwner, 1)
		pidOwnerChannelMap[pid] = chPidOwner
		go pidOwner(pid, chPidOwner)
	}
	return pidOwnerChannelMap
}

// iterative reducer
// iterates over channels and waits for them
func reducePidOwners(pidOwnerChannelMap map[int](chan PidOwner)) map[int]PidOwner {
	pidOwnersMap := map[int]PidOwner{}
	for _, ch := range pidOwnerChannelMap {
		for pidowner := range ch {
			if pidowner.err == nil {
				pidOwnersMap[pidowner.pid] = pidowner
			}
			close(ch)
		}
	}
	return pidOwnersMap
}

// reflect.select reducer
// selects a channel that has data using reflect.SelectCase
// because of the overhead of reflect.SelectCase, in this use case it's not really faster
func reducePidOwnersSelect(pidOwnerChannelMap map[int](chan PidOwner)) map[int]PidOwner {
	// // we need two matching arrays, one for select, another to look up pids by chosen channel index
	numCases := len(pidOwnerChannelMap)
	casesPidOwner := make([]reflect.SelectCase, numCases)
	pids := make([]int, numCases)
	i := 0
	for pid := range pidOwnerChannelMap {
		ch := pidOwnerChannelMap[pid]
		casesPidOwner[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
		pids[i] = pid
		i++
	}

	pidOwnersMap := map[int]PidOwner{}

	remainingPidOwner := len(casesPidOwner)
	for remainingPidOwner > 0 {
		chosen, recv, ok := reflect.Select(casesPidOwner)
		if !ok {
			fmt.Fprintf(os.Stderr, "reducePidOwnersSelect: Selected channel %d has been closed, zeroing out the channel to disable the case\n", chosen)
			casesPidOwner[chosen].Chan = reflect.ValueOf(nil)
			continue
		}

		pidowner := recv.Interface().(PidOwner)
		//fmt.Printf("Read from channel %d %#v and received %v\n", chosen, pidOwnerChannelMap[pids[chosen]], val)

		remainingPidOwner -= 1
		close(pidOwnerChannelMap[pids[chosen]])           // close channel
		casesPidOwner[chosen].Chan = reflect.ValueOf(nil) // zero out the channel to disable the case

		if pidowner.err == nil {
			//fmt.Printf("Setting owner for pid  %d\n", pidowner.pid)
			pidOwnersMap[pidowner.pid] = pidowner
		}
	}
	return pidOwnersMap
}
