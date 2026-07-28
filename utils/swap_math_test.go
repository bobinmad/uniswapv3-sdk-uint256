package utils

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/bobinmad/uniswapv3-sdk-uint256/constants"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/vuquang23/int256"
)

var swapStepCalculator = NewSwapStepCalculator()

func TestComputeSwapStep(t *testing.T) {

	p1 := EncodeSqrtRatioX96(big.NewInt(1), big.NewInt(1))
	p2 := EncodeSqrtRatioX96(big.NewInt(101), big.NewInt(100))
	p3 := EncodeSqrtRatioX96(big.NewInt(1000), big.NewInt(100))
	p4 := EncodeSqrtRatioX96(big.NewInt(10000), big.NewInt(100))

	tests := []struct {
		price       string
		priceTarget string
		liquidity   string
		amount      string
		fee         constants.FeeAmount

		expAmountIn  string
		expAmountOut string
		expFee       string

		expNextPrice string
	}{
		{p1.Dec(), p2.Dec(), "2000000000000000000", "1000000000000000000", 600,
			"9975124224178055", "9925619580021728", "5988667735148", "="},
		{p1.Dec(), p2.Dec(), "2000000000000000000", "-1000000000000000000", 600,
			"9975124224178055", "9925619580021728", "5988667735148", "="},

		{p1.Dec(), p3.Dec(), "2000000000000000000", "1000000000000000000", 600,
			"999400000000000000", "666399946655997866", "600000000000000", "<"},
		{p1.Dec(), p4.Dec(), "2000000000000000000", "-1000000000000000000", 600,
			"2000000000000000000", "1000000000000000000", "1200720432259356", "<"},

		{"417332158212080721273783715441582", "1452870262520218020823638996", "159344665391607089467575320103", "-1", 1,
			"1", "1", "1", "417332158212080721273783715441581"},

		{"2", "1", "1", "3915081100057732413702495386755767", 1,
			"39614081257132168796771975168", "0", "39614120871253040049813", "1"},

		{"2413", "79887613182836312", "1985041575832132834610021537970", "10", 1872,
			"0", "0", "10", "2413"},

		{"20282409603651670423947251286016", "22310650564016837466341976414617", "1024", "-4", 3000,
			"26215", "0", "79", "="},

		{"20282409603651670423947251286016", "18254168643286503381552526157414", "1024", "-263000", 3000,
			"1", "26214", "1", "="},
	}

	var sqrtRatioNextX96 Uint160
	var amountIn, amountOut, feeAmount Uint256
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			price := uint256.MustFromDecimal(tt.price)
			priceTarget := uint256.MustFromDecimal(tt.priceTarget)
			liquidity := uint256.MustFromDecimal(tt.liquidity)
			amount := int256.MustFromDec(tt.amount)
			fee := uint64(tt.fee)

			swapStepCalculator.ComputeSwapStep(
				price,
				priceTarget,
				liquidity,
				amount,
				fee,
				&sqrtRatioNextX96, &amountIn, &amountOut, &feeAmount,
				!price.Lt(priceTarget), amount.Sign() >= 0,
			)
			// require.Nil(t, err)

			if tt.expNextPrice == "=" {
				assert.Equal(t, tt.priceTarget, sqrtRatioNextX96.Dec())
			} else if tt.expNextPrice == "<" {
				assert.Greater(t, tt.priceTarget, sqrtRatioNextX96.Dec())
			} else {
				assert.Equal(t, tt.expNextPrice, sqrtRatioNextX96.Dec())
			}

			assert.Equal(t, tt.expAmountIn, amountIn.Dec())
			assert.Equal(t, tt.expAmountOut, amountOut.Dec())
			assert.Equal(t, tt.expFee, feeAmount.Dec())
		})
	}
}

func TestComputeSwapStepExactOutputAfterExactInputWithReusedAmount(t *testing.T) {
	calculator := NewSwapStepCalculator()
	price := EncodeSqrtRatioX96(big.NewInt(1), big.NewInt(1))
	priceTarget := EncodeSqrtRatioX96(big.NewInt(101), big.NewInt(100))
	liquidity := uint256.MustFromDecimal("2000000000000000000")
	amount := int256.MustFromDec("1000000000000000000")

	var sqrtRatioNextX96 Uint160
	var amountIn, amountOut, feeAmount Uint256
	calculator.ComputeSwapStep(
		price, priceTarget, liquidity, amount, 600,
		&sqrtRatioNextX96, &amountIn, &amountOut, &feeAmount,
		false, true,
	)

	// Pool переиспользует один и тот же Int256 между последовательными Swap.
	// Exact-output не должен менять signed amountRemaining через stale alias,
	// оставшийся в calculator после предыдущего exact-input.
	amount.SetFromDec("-1000000000000000000")
	calculator.ComputeSwapStep(
		price, priceTarget, liquidity, amount, 600,
		&sqrtRatioNextX96, &amountIn, &amountOut, &feeAmount,
		false, false,
	)

	assert.Equal(t, "-1000000000000000000", amount.Dec())
	assert.Equal(t, priceTarget.Dec(), sqrtRatioNextX96.Dec())
	assert.Equal(t, "9975124224178055", amountIn.Dec())
	assert.Equal(t, "9925619580021728", amountOut.Dec())
	assert.Equal(t, "5988667735148", feeAmount.Dec())
}

func TestComputeFeeAmountReducedRatioMatchesMulDiv(t *testing.T) {
	rng := rand.New(rand.NewSource(0xFEE_500))
	calculator := NewSwapStepCalculator()
	reference := NewFullMath()
	fees := []uint64{0, 1, 100, 500, 3_000, 10_000, 100_000}

	for trial := 0; trial < 20_000; trial++ {
		amountIn := &uint256.Int{rng.Uint64(), rng.Uint64(), 0, 0}
		feePips := fees[trial%len(fees)]
		calculator.setFeePips(feePips)

		var got, want uint256.Int
		calculator.computeFeeAmount(amountIn, &got)
		if err := reference.MulDivRoundingUpV2(
			amountIn,
			uint256.NewInt(feePips),
			uint256.NewInt(MaxFeeInt-feePips),
			&want,
		); err != nil {
			t.Fatalf("trial %d reference error: %v", trial, err)
		}
		if !got.Eq(&want) {
			t.Fatalf(
				"trial %d mismatch: amountIn=%s feePips=%d got=%s want=%s",
				trial,
				amountIn.Hex(),
				feePips,
				got.Hex(),
				want.Hex(),
			)
		}
	}
}
