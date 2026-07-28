package entities

import (
	"strconv"
	"testing"

	"github.com/holiman/uint256"
)

var benchmarkTickLiquidity uint256.Int
var benchmarkTickLookupIndex int

type fourWayTickCachePrevious struct {
	ticks     [4]int32
	indices   [4]int32
	validMask uint8
	next      uint8
}

func (c *fourWayTickCachePrevious) lookup(h *TicksHandler, tick int32, minIdx int) int {
	if mask := c.validMask; mask != 0 {
		if mask&1 != 0 && c.ticks[0] == tick {
			idx := int(c.indices[0])
			if idx >= minIdx && idx < h.TicksLen && h.Ticks[idx].Index == tick {
				return idx
			}
		} else if mask&2 != 0 && c.ticks[1] == tick {
			idx := int(c.indices[1])
			if idx >= minIdx && idx < h.TicksLen && h.Ticks[idx].Index == tick {
				return idx
			}
		} else if mask&4 != 0 && c.ticks[2] == tick {
			idx := int(c.indices[2])
			if idx >= minIdx && idx < h.TicksLen && h.Ticks[idx].Index == tick {
				return idx
			}
		} else if mask&8 != 0 && c.ticks[3] == tick {
			idx := int(c.indices[3])
			if idx >= minIdx && idx < h.TicksLen && h.Ticks[idx].Index == tick {
				return idx
			}
		}
	}

	lo, hi := minIdx, h.TicksLen
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if h.Ticks[mid].Index <= tick {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	idx := lo - 1
	if idx >= minIdx && h.Ticks[idx].Index == tick {
		slot := c.next & 3
		c.ticks[slot] = tick
		c.indices[slot] = int32(idx)
		c.validMask |= 1 << slot
		c.next = slot + 1
		return idx
	}
	return lo
}

// BenchmarkTicksHandlerUpdatePairs фиксирует реальные callers binarySearch:
// две границы исторического Mint и типичный Mint→Burn rebalance с попаданием
// Burn в exact-tick lookup cache.
func BenchmarkTicksHandlerUpdatePairs(b *testing.B) {
	pairs := make([][2]int32, 2_048)
	newPairs := make([][2]int32, len(pairs))
	handlerTemplate := newTickSearchBenchmarkHandler(4_096)
	var state uint32 = 0xc2b2ae35
	for i := range pairs {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		lowerIndex := int(state % uint32(len(handlerTemplate.Ticks)-64))
		pairs[i] = [2]int32{
			handlerTemplate.Ticks[lowerIndex].Index,
			handlerTemplate.Ticks[lowerIndex+51].Index,
		}
		newPairs[i] = [2]int32{
			handlerTemplate.Ticks[lowerIndex].Index + 1,
			handlerTemplate.Ticks[lowerIndex+51].Index + 1,
		}
	}
	liquidity := uint256.NewInt(1)

	b.Run("mint_existing_pairs", func(b *testing.B) {
		handler := newTickSearchBenchmarkHandler(4_096)
		mask := len(pairs) - 1
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pair := pairs[i&mask]
			handler.UpdateTicksAfterMint(pair[0], pair[1], liquidity)
		}
		benchmarkTickLiquidity = *handler.Ticks[2_048].LiquidityGross
	})

	b.Run("mint_burn_cached_pairs", func(b *testing.B) {
		handler := newTickSearchBenchmarkHandler(4_096)
		mask := len(pairs) - 1
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pair := pairs[i&mask]
			handler.UpdateTicksAfterMint(pair[0], pair[1], liquidity)
			handler.UpdateTicksAfterBurn(pair[0], pair[1], liquidity)
		}
		benchmarkTickLiquidity = *handler.Ticks[2_048].LiquidityGross
	})

	b.Run("mint_burn_new_pairs", func(b *testing.B) {
		handler := newTickSearchBenchmarkHandler(4_096)
		mask := len(newPairs) - 1
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pair := newPairs[i&mask]
			handler.UpdateTicksAfterMint(pair[0], pair[1], liquidity)
			handler.UpdateTicksAfterBurn(pair[0], pair[1], liquidity)
		}
		benchmarkTickLiquidity = *handler.Ticks[2_048].LiquidityGross
	})
}

// BenchmarkTicksHandlerBoundaryCacheCandidates моделирует несколько открытых
// Mint-пар до соответствующего Burn. Это тот случай, где четырёхэлементный
// fully-associative cache из полного профиля часто сохраняет tickUpper, но уже
// теряет tickLower. Direct-mapped production увеличивает working set без
// линейного сканирования всех cache slots на каждом miss.
func BenchmarkTicksHandlerBoundaryCacheCandidates(b *testing.B) {
	handlerTemplate := newTickSearchBenchmarkHandler(4_096)
	pairs := make([][2]int32, 2_048)
	var state uint32 = 0x27d4eb2d
	for i := range pairs {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		lowerIndex := int(state % uint32(len(handlerTemplate.Ticks)-64))
		pairs[i] = [2]int32{
			handlerTemplate.Ticks[lowerIndex].Index,
			handlerTemplate.Ticks[lowerIndex+51].Index,
		}
	}

	for _, distance := range []int{1, 4, 16} {
		b.Run("distance_"+strconv.Itoa(distance), func(b *testing.B) {
			b.Run("previous_4way", func(b *testing.B) {
				handler := newTickSearchBenchmarkHandler(4_096)
				var cache fourWayTickCachePrevious
				mask, result := len(pairs)-1, 0
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					mint := pairs[i&mask]
					lowerIdx := cache.lookup(handler, mint[0], 0)
					upperIdx := cache.lookup(handler, mint[1], lowerIdx+1)
					burn := pairs[(i-distance)&mask]
					lowerIdx = cache.lookup(handler, burn[0], 0)
					upperIdx = cache.lookup(handler, burn[1], lowerIdx)
					result += upperIdx
				}
				benchmarkTickLookupIndex = result
			})
			b.Run("production_direct_16way", func(b *testing.B) {
				handler := newTickSearchBenchmarkHandler(4_096)
				mask, result := len(pairs)-1, 0
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					mint := pairs[i&mask]
					_, lowerIdx, _ := handler.tickWithSliceKeyFrom(mint[0], 0)
					_, upperIdx, _ := handler.tickWithSliceKeyFrom(mint[1], lowerIdx+1)
					burn := pairs[(i-distance)&mask]
					_, lowerIdx, _ = handler.tickWithSliceKeyFrom(burn[0], 0)
					_, upperIdx, _ = handler.tickWithSliceKeyFrom(burn[1], lowerIdx)
					result += upperIdx
				}
				benchmarkTickLookupIndex = result
			})
		})
	}
}
