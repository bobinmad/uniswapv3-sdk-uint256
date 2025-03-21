package utils

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/daoleno/uniswap-sdk-core/entities"
	"github.com/holiman/uint256"
)

func TestMaxLiquidityForAmounts(t *testing.T) {
	type args struct {
		sqrtRatioCurrentX96 *uint256.Int
		sqrtRatioAX96       *uint256.Int
		sqrtRatioBX96       *uint256.Int
		amount0             *uint256.Int
		amount1             *uint256.Int
		useFullPrecision    bool
	}

	tmp, _ := new(big.Int).SetString("1214437677402050006470401421068302637228917309992228326090730924516431320489727", 10)
	lgamounts0 := uint256.MustFromBig(tmp)

	tmp, _ = new(big.Int).SetString("1214437677402050006470401421098959354205873606971497132040612572422243086574654", 10)
	lgamounts1 := uint256.MustFromBig(tmp)

	tmp, _ = new(big.Int).SetString("1214437677402050006470401421082903520362793114274352355276488318240158678126184", 10)
	lgamounts2 := uint256.MustFromBig(tmp)

	tests := []struct {
		name string
		args args
		want *uint256.Int
	}{
		{
			name: "imprecise - price inside - 100 token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				uint256.NewInt(200),
				false,
			},
			want: uint256.NewInt(2148),
		},
		{
			name: "imprecise - price inside - 100 token0, max token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				entities.MaxUint256U,
				false,
			},
			want: uint256.NewInt(2148),
		},
		{
			name: "imprecise - price inside - max token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				entities.MaxUint256U,
				uint256.NewInt(200),
				false,
			},
			want: uint256.NewInt(4297),
		},
		{
			name: "imprecise - price below - 100 token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(99), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				uint256.NewInt(200),
				false,
			},
			want: uint256.NewInt(1048),
		},
		{
			name: "imprecise - price below - 100 token0, max token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(99), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				entities.MaxUint256U,
				false,
			},
			want: uint256.NewInt(1048),
		},
		{
			name: "imprecise - price below - max token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(99), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				entities.MaxUint256U,
				uint256.NewInt(200),
				false,
			},
			want: lgamounts0,
		},
		{
			name: "imprecise - price above - 100 token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(111), uint256.NewInt(100)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				uint256.NewInt(200),
				false,
			},
			want: uint256.NewInt(2097),
		},
		{
			name: "imprecise - price above - 100 token0, max token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(111), uint256.NewInt(100)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				entities.MaxUint256U,
				false,
			},
			want: lgamounts1,
		},
		{
			name: "imprecise - price above - max token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(111), uint256.NewInt(100)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				entities.MaxUint256U,
				uint256.NewInt(200),
				false,
			},
			want: uint256.NewInt(2097),
		},
		{
			name: "precise - price inside - 100 token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				uint256.NewInt(200),
				true,
			},
			want: uint256.NewInt(2148),
		},
		{
			name: "precise - price inside - 100 token0, max token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				entities.MaxUint256U,
				true,
			},
			want: uint256.NewInt(2148),
		},
		{
			name: "precise - price inside - max token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(1), uint256.NewInt(1)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				entities.MaxUint256U,
				uint256.NewInt(200),
				true,
			},
			want: uint256.NewInt(4297),
		},
		{
			name: "precise - price below - 100 token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(99), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				uint256.NewInt(200),
				true,
			},
			want: uint256.NewInt(1048),
		},
		{
			name: "precise - price below - 100 token0, max token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(99), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				entities.MaxUint256U,
				true,
			},
			want: uint256.NewInt(1048),
		},
		{
			name: "precise - price below - max token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(99), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				entities.MaxUint256U,
				uint256.NewInt(200),
				true,
			},
			want: lgamounts2,
		},
		{
			name: "precise - price above - 100 token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(111), uint256.NewInt(100)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				uint256.NewInt(200),
				true,
			},
			want: uint256.NewInt(2097),
		},
		{
			name: "precise - price above - 100 token0, max token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(111), uint256.NewInt(100)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				uint256.NewInt(100),
				entities.MaxUint256U,
				true,
			},
			want: lgamounts1,
		},
		{
			name: "precise - price above - max token0, 200 token1",
			args: args{
				EncodeSqrtRatioX96(uint256.NewInt(111), uint256.NewInt(100)),
				EncodeSqrtRatioX96(uint256.NewInt(100), uint256.NewInt(110)),
				EncodeSqrtRatioX96(uint256.NewInt(110), uint256.NewInt(100)),
				entities.MaxUint256U,
				uint256.NewInt(200),
				true,
			},
			want: uint256.NewInt(2097),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaxLiquidityForAmounts(tt.args.sqrtRatioCurrentX96, tt.args.sqrtRatioAX96, tt.args.sqrtRatioBX96, tt.args.amount0, tt.args.amount1, tt.args.useFullPrecision); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("maxLiquidityForAmounts() = %v, want %v", got, tt.want)
			}
		})
	}
}
