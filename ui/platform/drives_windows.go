//go:build windows

package platform

import (
	"sort"

	"golang.org/x/sys/windows"
)

var (
	kernel32ProcGetLogicalDrives = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetLogicalDrives")
)

func AvailableLocalDrives() []string {
	ret, _, _ := kernel32ProcGetLogicalDrives.Call()
	mask := uint32(ret)
	if mask == 0 {
		return nil
	}
	out := make([]string, 0, 8)
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		out = append(out, string(rune('A'+i))+":\\")
	}
	sort.Strings(out)
	return out
}
