package utils

import (
	"math/bits"

	"github.com/holiman/uint256"
)

// var ErrInvalidInput = errors.New("invalid input")

func MostSignificantBit(x *uint256.Int) uint {
	if x[3] != 0 {
		return 192 + uint(63-bits.LeadingZeros64(x[3]))
	}
	if x[2] != 0 {
		return 128 + uint(63-bits.LeadingZeros64(x[2]))
	}
	if x[1] != 0 {
		return 64 + uint(63-bits.LeadingZeros64(x[1]))
	}
	if x[0] != 0 {
		return uint(63 - bits.LeadingZeros64(x[0]))
	}
	return 0
}
