//go:build !windows

package delegation

import (
	"context"
	"errors"
)

func observeSystemBoot(context.Context) (bootObservation, error) {
	return bootObservation{}, errors.New("trusted boot evidence is not supported on this platform; workspace remains fenced")
}
