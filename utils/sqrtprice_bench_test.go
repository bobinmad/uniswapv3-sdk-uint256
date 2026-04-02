package utils

import (
	"testing"

	"github.com/bobinmad/uniswapv3-sdk-uint256/constants"
	"github.com/holiman/uint256"
)

// oldGetNextSqrtPriceFromAmount0RoundingUp — original implementation for comparison.
func oldGetNextSqrtPriceFromAmount0RoundingUp(c *SqrtPriceCalculator, sqrtPX96 *Uint160, liquidity *Uint128, amount *uint256.Int, add bool, result *Uint160) error {
	if amount.IsZero() {
		result.Set(sqrtPX96)
		return nil
	}

	c.numerator1.Lsh(liquidity, 96)
	c.product.Mul(amount, sqrtPX96)

	if add {
		if c.tmp.Div(c.product, amount).Eq(sqrtPX96) {
			if !c.deno.Add(c.numerator1, c.product).Lt(c.numerator1) {
				return c.fullMath.MulDivRoundingUpV2(c.numerator1, sqrtPX96, c.deno, result)
			}
		}

		c.deno.Div(c.numerator1, sqrtPX96)
		c.deno.Add(c.deno, amount)
		c.fullMath.DivRoundingUp(c.numerator1, c.deno, result)
		return nil
	}
	if !c.tmp.Div(c.product, amount).Eq(sqrtPX96) {
		return ErrInvariant
	}
	if !c.numerator1.Gt(c.product) {
		return ErrInvariant
	}
	return c.fullMath.MulDivRoundingUpV2(c.numerator1, sqrtPX96, c.deno.Sub(c.numerator1, c.product), result)
}

// oldGetNextSqrtPriceFromAmount1RoundingDown — original implementation for comparison.
func oldGetNextSqrtPriceFromAmount1RoundingDown(c *SqrtPriceCalculator, sqrtPX96 *Uint160, liquidity *Uint128, amount *uint256.Int, add bool, result *Uint160) error {
	if add {
		if !amount.Gt(MaxUint160) {
			result.Lsh(amount, 96)
			result.Div(result, liquidity)
		} else {
			result.Div(c.tmp.Mul(amount, constants.Q96U256), liquidity)
		}
		if _, overflow := result.AddOverflow(result, sqrtPX96); overflow {
			return ErrAddOverflow
		}
		return nil
	}
	if err := c.fullMath.MulDivRoundingUpV2(amount, constants.Q96U256, liquidity, result); err != nil {
		return err
	}
	if !sqrtPX96.Gt(result) {
		return ErrInvariant
	}
	result.Sub(sqrtPX96, result)
	return nil
}

var benchCases = []struct {
	name      string
	sqrtPX96  *uint256.Int
	liquidity *uint256.Int
	amount    *uint256.Int
	add       bool
}{
	// add=true, fast path (no overflow in product)
	{
		"add=true/fast",
		uint256.MustFromDecimal("79228162514264337593543950336"), // 1.0 Q96
		uint256.MustFromHex("0xde0b6b3a7640000"),
		uint256.MustFromHex("0x16345785d8a0000"),
		true,
	},
	// add=true, overflow path (product overflows)
	{
		"add=true/overflow",
		uint256.MustFromHex("0xffffffffffffffffffffffffffffffffffffffff"),
		uint256.MustFromHex("0xffffffffffffffffffffffffffffffff"),
		uint256.MustFromHex("0x1"),
		true,
	},
	// add=false
	{
		"add=false",
		uint256.MustFromDecimal("1025574284609383690408304870162715216695788925244"),
		uint256.MustFromDecimal("50015962439936049619261659728067971248"),
		uint256.MustFromDecimal("406"),
		false,
	},
}

var benchCases1 = []struct {
	name      string
	sqrtPX96  *uint256.Int
	liquidity *uint256.Int
	amount    *uint256.Int
	add       bool
}{
	// add=true, amount <= MaxUint160 (fast path)
	{
		"add=true/small",
		uint256.MustFromDecimal("79228162514264337593543950336"),
		uint256.MustFromHex("0xde0b6b3a7640000"),
		uint256.MustFromHex("0x16345785d8a0000"),
		true,
	},
	// add=true, amount > MaxUint160 (else path)
	{
		"add=true/large",
		uint256.MustFromDecimal("79228162514264337593543950336"),
		uint256.MustFromHex("0xde0b6b3a7640000"),
		uint256.MustFromHex("0x10000000000000000000000000000000000000001"), // > MaxUint160
		true,
	},
	// add=false
	{
		"add=false",
		uint256.MustFromDecimal("79228162514264337593543950336"),
		uint256.MustFromHex("0xde0b6b3a7640000"),
		uint256.MustFromHex("0x16345785d8a0000"),
		false,
	},
}

func BenchmarkGetNextSqrtPrice_Amount1_New(b *testing.B) {
	c := NewSqrtPriceCalculator()
	var r Uint160
	for _, bc := range benchCases1 {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = c.getNextSqrtPriceFromAmount1RoundingDown(bc.sqrtPX96, bc.liquidity, bc.amount, bc.add, &r)
			}
		})
	}
}

func BenchmarkGetNextSqrtPrice_Amount1_Old(b *testing.B) {
	c := NewSqrtPriceCalculator()
	var r Uint160
	for _, bc := range benchCases1 {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = oldGetNextSqrtPriceFromAmount1RoundingDown(c, bc.sqrtPX96, bc.liquidity, bc.amount, bc.add, &r)
			}
		})
	}
}

func BenchmarkGetNextSqrtPrice_Amount0_New(b *testing.B) {
	c := NewSqrtPriceCalculator()
	var r Uint160
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = c.getNextSqrtPriceFromAmount0RoundingUp(bc.sqrtPX96, bc.liquidity, bc.amount, bc.add, &r)
			}
		})
	}
}

func BenchmarkGetNextSqrtPrice_Amount0_Old(b *testing.B) {
	c := NewSqrtPriceCalculator()
	var r Uint160
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = oldGetNextSqrtPriceFromAmount0RoundingUp(c, bc.sqrtPX96, bc.liquidity, bc.amount, bc.add, &r)
			}
		})
	}
}
