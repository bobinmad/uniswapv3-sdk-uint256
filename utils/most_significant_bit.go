package utils

import (
	"errors"

	"github.com/holiman/uint256"
)

var ErrInvalidInput = errors.New("invalid input")

type powerOf2 struct {
	power uint
	value *uint256.Int
}

type BitCalculator struct {
	tmpX *uint256.Int
}

func NewBitCalculator() *BitCalculator {
	return &BitCalculator{
		tmpX: new(uint256.Int),
	}
}

var powersOf2 = []powerOf2{
	{128, uint256.MustFromHex("0x100000000000000000000000000000000")},
	{64, uint256.MustFromHex("0x10000000000000000")},
	{32, uint256.MustFromHex("0x100000000")},
	{16, uint256.MustFromHex("0x10000")},
	{8, uint256.MustFromHex("0x100")},
	{4, uint256.MustFromHex("0x10")},
	{2, uint256.MustFromHex("0x4")},
	{1, uint256.MustFromHex("0x2")},
}

func (c *BitCalculator) MostSignificantBit(x *uint256.Int) (uint, error) {
	if x.IsZero() {
		return 0, ErrInvalidInput
	}

	c.tmpX.Set(x)
	var msb uint
	for _, p := range powersOf2 {
		if !c.tmpX.Lt(p.value) {
			c.tmpX.Rsh(c.tmpX, p.power)
			msb += p.power
		}
	}

	return msb, nil
}
