//go:build !windows

package platform

func AvailableLocalDrives() []string {
	return nil
}
