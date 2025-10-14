package entities

import (
	"testing"

	"github.com/KyberNetwork/int256"
	"github.com/bobinmad/uniswapv3-sdk-uint256/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
)

var (
	lowTick = Tick{
		Index:          utils.MinTick + 1,
		LiquidityNet:   int256.NewInt(10),
		LiquidityGross: uint256.NewInt(10),
	}
	midTick = Tick{
		Index:          0,
		LiquidityNet:   int256.NewInt(-5),
		LiquidityGross: uint256.NewInt(5),
	}
	highTick = Tick{
		Index:          utils.MaxTick - 1,
		LiquidityNet:   int256.NewInt(-5),
		LiquidityGross: uint256.NewInt(5),
	}
)

func TestValidateList(t *testing.T) {
	assert.ErrorIs(t, ValidateList([]Tick{lowTick}, 1), ErrZeroNet, "panics for incomplete lists")
	assert.ErrorIs(t, ValidateList([]Tick{highTick, lowTick, midTick}, 1), ErrSorted, "panics for unsorted lists")
	assert.ErrorIs(t, ValidateList([]Tick{highTick, midTick, lowTick}, 1337), ErrInvalidTickSpacing, "errors if ticks are not on multiples of tick spacing")
}

func TestIsBelowSmallest(t *testing.T) {
	result := []Tick{lowTick, midTick, highTick}
	isBelowSmallest1, _ := IsBelowSmallest(result, utils.MinTick)
	assert.True(t, isBelowSmallest1)

	isBelowSmallest2, _ := IsBelowSmallest(result, utils.MinTick+1)
	assert.False(t, isBelowSmallest2)
}

func TestIsAtOrAboveSmallest(t *testing.T) {
	result := []Tick{lowTick, midTick, highTick}

	isAtOrAboveLargest1, _ := IsAtOrAboveLargest(result, utils.MaxTick-2)
	assert.False(t, isAtOrAboveLargest1)

	isAtOrAboveLargest2, _ := IsAtOrAboveLargest(result, utils.MaxTick-1)
	assert.True(t, isAtOrAboveLargest2)
}

func TestNextInitializedTick(t *testing.T) {
	ticks := []Tick{lowTick, midTick, highTick}

	type args struct {
		ticks []Tick
		tick  int32
		lte   bool
	}
	tests := []struct {
		name string
		args args
		want Tick
	}{
		{name: "low - lte = true 0", args: args{ticks: ticks, tick: utils.MinTick + 1, lte: true}, want: lowTick},
		{name: "low - lte = true 1", args: args{ticks: ticks, tick: utils.MinTick + 2, lte: true}, want: lowTick},
		{name: "low - lte = false 0", args: args{ticks: ticks, tick: utils.MinTick, lte: false}, want: lowTick},
		{name: "low - lte = false 1", args: args{ticks: ticks, tick: utils.MinTick + 1, lte: false}, want: midTick},
		{name: "mid - lte = true 0", args: args{ticks: ticks, tick: 0, lte: true}, want: midTick},
		{name: "mid - lte = true 1", args: args{ticks: ticks, tick: 1, lte: true}, want: midTick},
		{name: "mid - lte = false 0", args: args{ticks: ticks, tick: -1, lte: false}, want: midTick},
		{name: "mid - lte = false 1", args: args{ticks: ticks, tick: 0 + 1, lte: false}, want: highTick},
		{name: "high - lte = true 0", args: args{ticks: ticks, tick: utils.MaxTick - 1, lte: true}, want: highTick},
		{name: "high - lte = true 1", args: args{ticks: ticks, tick: utils.MaxTick, lte: true}, want: highTick},
		{name: "high - lte = false 0", args: args{ticks: ticks, tick: utils.MaxTick - 2, lte: false}, want: highTick},
		{name: "high - lte = false 1", args: args{ticks: ticks, tick: utils.MaxTick - 3, lte: false}, want: highTick},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextInitializedTick, _ := NextInitializedTick(tt.args.ticks, tt.args.tick, tt.args.lte)
			assert.Equal(t, tt.want, nextInitializedTick)
		})
	}

	nextInitializedTick1, err1 := NextInitializedTick(ticks, utils.MinTick, true)
	assert.Zero(t, nextInitializedTick1, "below smallest")
	assert.ErrorIs(t, err1, ErrBelowSmallest)

	nextInitializedTick2, err2 := NextInitializedTick(ticks, utils.MaxTick-1, false)
	assert.Zero(t, nextInitializedTick2, "at or above largest")
	assert.ErrorIs(t, err2, ErrAtOrAboveLargest)
}

