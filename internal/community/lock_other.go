//go:build !darwin && !linux

package community

import "errors"

func lock(string) (func(), error) {
	return nil, errors.New("community bundle installation requires macOS or Linux")
}
