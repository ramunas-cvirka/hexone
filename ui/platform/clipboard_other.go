//go:build !windows

package platform

import "errors"

func ReadClipboardTextNow() (string, error) {
	return "", errors.New("synchronous clipboard read unsupported")
}
