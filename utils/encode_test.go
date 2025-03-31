package utils

import (
	"testing"

	"github.com/KyberNetwork/uniswapv3-sdk-uint256/constants"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
)

func TestEncodeSqrtRatioX96(t *testing.T) {
	assert.Equal(t, EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1)), constants.Q96U256, "1/1")

	r0 := uint256.MustFromDecimal("792281625142643375935439503360")
	assert.Equal(t, EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(1)), r0, 10, "100/1")

	r1 := uint256.MustFromDecimal("7922816251426433759354395033")
	assert.Equal(t, EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(100)), r1, 10, "1/100")

	r2 := uint256.MustFromDecimal("45742400955009932534161870629")
	assert.Equal(t, EncodeSqrtRatioX96(uint256.NewInt(111), uint256.NewInt(333)), r2, 10, "111/333")

	r3 := uint256.MustFromDecimal("137227202865029797602485611888")
	assert.Equal(t, EncodeSqrtRatioX96(uint256.NewInt(333), uint256.NewInt(111)), r3, 10, "333/111")
}
