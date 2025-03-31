package utils

import (
	"fmt"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sqrtPriceCalculator = NewSqrtPriceCalculator()

func TestGetNextSqrtPriceFromInput(t *testing.T) {
	// https://github.com/Uniswap/v3-core/blob/main/test/SqrtPriceMath.spec.ts
	p1 := EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1))

	tests := []struct {
		price      string
		liquidity  string
		amount     string
		zeroForOne bool
		expResult  string
	}{
		{"0x1", "0x1", "0x8000000000000000000000000000000000000000000000000000000000000000", true, "1"},
		{p1.Hex(), "0x16345785d8a0000", "0x0", true, p1.Dec()},
		{p1.Hex(), "0x16345785d8a0000", "0x0", false, p1.Dec()},
		{"0xffffffffffffffffffffffffffffffffffffffff", "0xffffffffffffffffffffffffffffffff", "0xfffffffffffffffffffffffffffffffffffffffeffffffffffffffffffffffff", true, "1"},
		{p1.Hex(), "0xde0b6b3a7640000", "0x16345785d8a0000", false, "87150978765690771352898345369"},
		{p1.Hex(), "0xde0b6b3a7640000", "0x16345785d8a0000", true, "72025602285694852357767227579"},
		{p1.Hex(), "0x8ac7230489e80000", "0x10000000000000000000000000", true, "624999999995069620"},
		{p1.Hex(), "0x1", "0x8000000000000000000000000000000000000000000000000000000000000000", true, "1"},
	}

	var r Uint160
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			err := sqrtPriceCalculator.GetNextSqrtPriceFromInput(
				uint256.MustFromHex(tt.price), uint256.MustFromHex(tt.liquidity),
				uint256.MustFromHex(tt.amount), tt.zeroForOne, &r)
			require.Nil(t, err)
			assert.Equal(t, tt.expResult, r.Dec())
		})
	}

	failTests := []struct {
		price      string
		liquidity  string
		amount     string
		zeroForOne bool
	}{
		{"0x0", "0x1", "0x16345785d8a0000", false},
		{"0x1", "0x0", "0x16345785d8a0000", true},
	}
	for i, tt := range failTests {
		t.Run(fmt.Sprintf("fail test %d", i), func(t *testing.T) {
			err := sqrtPriceCalculator.GetNextSqrtPriceFromInput(
				uint256.MustFromHex(tt.price), uint256.MustFromHex(tt.liquidity),
				uint256.MustFromHex(tt.amount), tt.zeroForOne, &r)
			require.NotNil(t, err)
		})
	}
}

func TestGetNextSqrtPriceFromOutput(t *testing.T) {
	// https://github.com/Uniswap/v3-core/blob/main/test/SqrtPriceMath.spec.ts
	p1 := EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1))

	tests := []struct {
		price      string
		liquidity  string
		amount     string
		zeroForOne bool
		expResult  string
	}{
		{"0x100000000000000000000000000", "0x400", "0x3ffff", true, "77371252455336267181195264"},
		{p1.Hex(), "0x16345785d8a0000", "0x0", true, p1.Dec()},
		{p1.Hex(), "0x16345785d8a0000", "0x0", false, p1.Dec()},
		{p1.Hex(), "0xde0b6b3a7640000", "0x16345785d8a0000", false, "88031291682515930659493278152"},
		{p1.Hex(), "0xde0b6b3a7640000", "0x16345785d8a0000", true, "71305346262837903834189555302"},
	}
	var r Uint160
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			err := sqrtPriceCalculator.GetNextSqrtPriceFromOutput(
				uint256.MustFromHex(tt.price), uint256.MustFromHex(tt.liquidity),
				uint256.MustFromHex(tt.amount), tt.zeroForOne, &r)
			require.Nil(t, err)
			assert.Equal(t, tt.expResult, r.Dec())
		})
	}

	failTests := []struct {
		price      string
		liquidity  string
		amount     string
		zeroForOne bool
	}{
		{"0x0", "0x1", "0x16345785d8a0000", false},
		{"0x1", "0x0", "0x16345785d8a0000", true},

		// TODO: fix broken test
		{"0x100000000000000000000000000", "0x400", "0x4", false}, // output amount is exactly the virtual reserves of token0

		{"0x100000000000000000000000000", "0x400", "0x5", false},    // output amount is greater than virtual reserves of token0
		{"0x100000000000000000000000000", "0x400", "0x40001", true}, // output amount is greater than virtual reserves of token1
		{"0x100000000000000000000000000", "0x400", "0x40000", true}, // output amount is exactly the virtual reserves of token1

		{p1.Hex(), "0x1", MaxUint256.Hex(), true},  // amountOut is impossible in zero for one direction
		{p1.Hex(), "0x1", MaxUint256.Hex(), false}, // amountOut is impossible in one for zero direction
	}
	for i, tt := range failTests {
		t.Run(fmt.Sprintf("fail test %d", i), func(t *testing.T) {
			err := sqrtPriceCalculator.GetNextSqrtPriceFromOutput(
				uint256.MustFromHex(tt.price), uint256.MustFromHex(tt.liquidity),
				uint256.MustFromHex(tt.amount), tt.zeroForOne, &r)
			require.NotNil(t, err)
		})
	}
}

func TestGetAmount0Delta(t *testing.T) {
	// https://github.com/Uniswap/v3-core/blob/main/test/SqrtPriceMath.spec.ts
	p1 := EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1))
	p2 := EncodeSqrtRatioX96(uint256.NewInt(2), uint256.NewInt(1))
	p3 := EncodeSqrtRatioX96(uint256.NewInt(121), uint256.NewInt(100))

	p4 := EncodeSqrtRatioX96(new(uint256.Int).Exp(uint256.NewInt(2), uint256.NewInt(90)), uint256.NewInt(1))
	p5 := EncodeSqrtRatioX96(new(uint256.Int).Exp(uint256.NewInt(2), uint256.NewInt(96)), uint256.NewInt(1))

	tests := []struct {
		price      string
		liquidity  string
		amount     string
		zeroForOne bool
		expResult  string
	}{
		{p1.Hex(), p2.Hex(), "0x0", true, "0"},
		{p1.Hex(), p1.Hex(), "0x1", true, "0"},
		{p1.Hex(), p3.Hex(), "0xde0b6b3a7640000", true, "90909090909090910"},
		{p1.Hex(), p3.Hex(), "0xde0b6b3a7640000", false, "90909090909090909"},
		{p4.Hex(), p5.Hex(), "0xde0b6b3a7640000", true, "24869"},
		{p4.Hex(), p5.Hex(), "0xde0b6b3a7640000", false, "24868"},
	}
	var r Uint256
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			err := sqrtPriceCalculator.GetAmount0DeltaV2(
				uint256.MustFromHex(tt.price), uint256.MustFromHex(tt.liquidity),
				uint256.MustFromHex(tt.amount), tt.zeroForOne, &r)
			require.Nil(t, err)
			assert.Equal(t, tt.expResult, r.Dec())
		})
	}
}

func TestGetAmount1Delta(t *testing.T) {
	// https://github.com/Uniswap/v3-core/blob/main/test/SqrtPriceMath.spec.ts
	p1 := EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1))
	p2 := EncodeSqrtRatioX96(uint256.NewInt(2), uint256.NewInt(1))
	p3 := EncodeSqrtRatioX96(uint256.NewInt(121), uint256.NewInt(100))
	p4 := EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(121))

	tests := []struct {
		price      string
		liquidity  string
		amount     string
		zeroForOne bool
		expResult  string
	}{
		{p1.Hex(), p2.Hex(), "0x0", true, "0"},
		{p1.Hex(), p1.Hex(), "0x1", true, "0"},
		{p1.Hex(), p3.Hex(), "0xde0b6b3a7640000", true, "100000000000000000"},
		{p1.Hex(), p3.Hex(), "0xde0b6b3a7640000", false, "99999999999999999"},
		{p4.Hex(), p1.Hex(), "0xde0b6b3a7640000", true, "90909090909090910"},
		{p4.Hex(), p1.Hex(), "0xde0b6b3a7640000", false, "90909090909090909"},
	}
	var r Uint256
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			err := sqrtPriceCalculator.GetAmount1DeltaV2(
				uint256.MustFromHex(tt.price), uint256.MustFromHex(tt.liquidity),
				uint256.MustFromHex(tt.amount), tt.zeroForOne, &r)
			require.Nil(t, err)
			assert.Equal(t, tt.expResult, r.Dec())
		})
	}
}

func TestSwap(t *testing.T) {
	// sqrtP * sqrtQ overflows

	var sqrtQ Uint160
	err := sqrtPriceCalculator.GetNextSqrtPriceFromInput(
		uint256.MustFromDecimal("1025574284609383690408304870162715216695788925244"),
		uint256.MustFromDecimal("50015962439936049619261659728067971248"),
		uint256.MustFromDecimal("406"), true, &sqrtQ)
	require.Nil(t, err)

	require.Equal(t, "1025574284609383582644711336373707553698163132913", sqrtQ.Dec())

	var amount0Delta Uint256
	err = sqrtPriceCalculator.GetAmount0DeltaV2(
		&sqrtQ,
		uint256.MustFromDecimal("1025574284609383690408304870162715216695788925244"),
		uint256.MustFromDecimal("50015962439936049619261659728067971248"), true, &amount0Delta)
	require.Nil(t, err)

	assert.Equal(t, "406", amount0Delta.Dec())
}
