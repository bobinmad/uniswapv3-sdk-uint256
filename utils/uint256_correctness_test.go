// Copyright 2026 uniswapv3-sdk-uint256 authors.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"math/big"
	"math/bits"
	"math/rand"
	"slices"
	"testing"

	"github.com/holiman/uint256"
)

// TestUdivremBy1WithRecipMatchesMathBig — фаззит udivremBy1WithRecip против
// math/big.Int.QuoRem на нормализованных входах. Этот тест добавлен после того,
// как разработка ASM-версии (откатанной как net regression — см. историю)
// случайно вскрыла pre-existing баг в udivrem2by1: вторая коррекция (`if r >= d`)
// была ошибочно вложена внутрь первой (`if r > ql`), что нарушало
// Möller–Granlund Algorithm 4. Сейчас тест служит regression-guard.
func TestUdivremBy1WithRecipMatchesMathBig(t *testing.T) {
	rng := rand.New(rand.NewSource(0xC0FFEE))
	const trials = 20_000
	for trial := 0; trial < trials; trial++ {
		var d uint64
		for d == 0 {
			d = rng.Uint64()
		}
		shift := uint(bits.LeadingZeros64(d))
		dn := d << shift
		recip := reciprocal2by1(dn)

		uLen := 1 + rng.Intn(7)
		raw := make([]uint64, uLen)
		for i := range raw {
			raw[i] = rng.Uint64()
		}
		un := make([]uint64, uLen+1)
		un[uLen] = raw[uLen-1] >> (64 - shift)
		for i := uLen - 1; i > 0; i-- {
			un[i] = (raw[i] << shift) | (raw[i-1] >> (64 - shift))
		}
		un[0] = raw[0] << shift

		quot := make([]uint64, uLen+2)
		unCopy := slices.Clone(un)
		rem := udivremBy1WithRecip(quot, unCopy, dn, recip)

		uBig := new(big.Int)
		for i := uLen - 1; i >= 0; i-- {
			uBig.Lsh(uBig, 64)
			uBig.Or(uBig, new(big.Int).SetUint64(raw[i]))
		}
		dBig := new(big.Int).SetUint64(d)
		qBig, rBig := new(big.Int).QuoRem(uBig, dBig, new(big.Int))

		got := new(big.Int)
		for i := uLen + 1; i >= 0; i-- {
			got.Lsh(got, 64)
			got.Or(got, new(big.Int).SetUint64(quot[i]))
		}
		gotRem := rem >> shift
		if got.Cmp(qBig) != 0 || gotRem != rBig.Uint64() {
			t.Fatalf("trial %d mismatch vs math/big:\n  raw=%v d=%x\n  truth: q=%s r=%s\n  got:   q=%s r=%d",
				trial, raw, d, qBig.Text(16), rBig.Text(16), got.Text(16), gotRem)
		}
	}
}

// FuzzUdivremBy1WithRecipMatchesMathBig — go test -fuzz target.
// Запуск:
//
//	go test -run=^$ -fuzz=FuzzUdivremBy1WithRecipMatchesMathBig -fuzztime=60s ./utils/
//
// Seed corpus содержит известный tricky case, который вскрыл нестед-if баг
// в udivrem2by1 (см. комментарий в udivrem2by1 / TestUdivremBy1WithRecipMatchesMathBig).
func FuzzUdivremBy1WithRecipMatchesMathBig(f *testing.F) {
	f.Add(uint64(0), uint64(0), uint64(0), uint64(0), uint64(0xb273c87e43bebdc2), uint64(0xd6ba), uint64(0x431))
	f.Add(uint64(0), uint64(0), uint64(0), uint64(0), uint64(0), uint64(1), uint64(0xffffffffffffffff))
	f.Add(uint64(0xffffffffffffffff), uint64(0xffffffffffffffff), uint64(0xffffffffffffffff), uint64(0xffffffffffffffff), uint64(0xffffffffffffffff), uint64(0xffffffffffffffff), uint64(0x8000000000000001))
	f.Add(uint64(1), uint64(2), uint64(3), uint64(4), uint64(5), uint64(6), uint64(0xff))

	f.Fuzz(func(t *testing.T, u0, u1, u2, u3, u4, u5, d uint64) {
		if d == 0 {
			t.Skip()
		}
		raw := []uint64{u0, u1, u2, u3, u4, u5}
		uLen := len(raw)
		shift := uint(bits.LeadingZeros64(d))
		dn := d << shift
		recip := reciprocal2by1(dn)

		un := make([]uint64, uLen+1)
		un[uLen] = raw[uLen-1] >> (64 - shift)
		for i := uLen - 1; i > 0; i-- {
			un[i] = (raw[i] << shift) | (raw[i-1] >> (64 - shift))
		}
		un[0] = raw[0] << shift

		quot := make([]uint64, uLen+2)
		unCopy := slices.Clone(un)
		rem := udivremBy1WithRecip(quot, unCopy, dn, recip)

		uBig := new(big.Int)
		for i := uLen - 1; i >= 0; i-- {
			uBig.Lsh(uBig, 64)
			uBig.Or(uBig, new(big.Int).SetUint64(raw[i]))
		}
		dBig := new(big.Int).SetUint64(d)
		qBig, rBig := new(big.Int).QuoRem(uBig, dBig, new(big.Int))

		got := new(big.Int)
		for i := uLen + 1; i >= 0; i-- {
			got.Lsh(got, 64)
			got.Or(got, new(big.Int).SetUint64(quot[i]))
		}
		gotRem := rem >> shift
		if got.Cmp(qBig) != 0 || gotRem != rBig.Uint64() {
			t.Fatalf("mismatch vs math/big:\n  raw=%v d=%x\n  truth: q=%s r=%s\n  got:   q=%s r=%d",
				raw, d, qBig.Text(16), rBig.Text(16), got.Text(16), gotRem)
		}
	})
}

// TestMulDivV2Roundtrip — end-to-end интеграционный тест: сравнивает наш
// FullMath.MulDivV2 (использующий udivrem) с эталоном holiman MulDivOverflow
// на ~5k псевдо-случайных входов разных размеров. Ловит регрессии на
// уровне всей цепочки делителя, включая spec-cases dLen=1/2/3/4.
func TestMulDivV2Roundtrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	fm := NewFullMath()
	for trial := 0; trial < 5000; trial++ {
		a := randUintForRoundtrip(rng)
		b := randUintForRoundtrip(rng)
		d := randUintForRoundtrip(rng)
		if d.IsZero() {
			continue
		}
		ref := new(uint256.Int)
		ref, of := ref.MulDivOverflow(a, b, d)
		var got uint256.Int
		err := fm.MulDivV2(a, b, d, &got, nil)
		if of {
			if err == nil {
				t.Fatalf("trial %d: expected overflow err for a=%s b=%s d=%s", trial, a.Hex(), b.Hex(), d.Hex())
			}
			continue
		}
		if err != nil {
			t.Fatalf("trial %d: unexpected err %v for a=%s b=%s d=%s", trial, err, a.Hex(), b.Hex(), d.Hex())
		}
		if !ref.Eq(&got) {
			t.Fatalf("trial %d: mismatch\n  a=%s\n  b=%s\n  d=%s\n  ref=%s\n  got=%s",
				trial, a.Hex(), b.Hex(), d.Hex(), ref.Hex(), got.Hex())
		}
	}
}

// TestUdivremRemHigherWordsCleared — regression-guard: до фикса
// (см. комментарий в udivrem case 1/2/3) старшие слова `rem` могли остаться
// stale-данными от предыдущего вызова, если предыдущий вызов имел больший
// dLen. Это приводило к неверному результату MulDivRoundingUpV2 →
// неверному GetAmount0DeltaV2 → ошибочному выводу из ComputeSwapStep.
// Тест эмулирует sequence: dLen=4 → dLen=1 → проверяет, что rem.IsZero()
// возвращает корректный ответ.
func TestUdivremRemHigherWordsCleared(t *testing.T) {
	ut := NewUint256Utils()

	// 1) Сначала дешим на 4-словесный делитель, чтобы rem заполнился во всех 4 словах.
	d4 := uint256.MustFromHex("0x1000000000000000200000000000000030000000000000004000000000000007")
	u := []uint64{
		0xdeadbeefdeadbeef, 0xcafebabecafebabe, 0xfeedfacefeedface, 0xbaadf00dbaadf00d,
		0x1234567890abcdef, 0x0fedcba987654321, 0xa5a5a5a5a5a5a5a5, 0x5a5a5a5a5a5a5a5a,
	}
	quot := make([]uint64, 8)
	rem := new(uint256.Int)
	ut.udivrem(quot, u, d4, rem)
	// rem теперь имеет ненулевые старшие слова (rem[2], rem[3]).

	// 2) Теперь делим на 1-словесный делитель. Если фикса нет — rem[1..3] останутся stale.
	d1 := uint256.NewInt(1_000_000_000)
	for i := range quot {
		quot[i] = 0
	}
	uSmall := []uint64{0xabcdef0123456789, 0x0000000000000007, 0, 0, 0, 0, 0, 0}
	ut.udivrem(quot, uSmall, d1, rem)

	// 3) Проверяем правильность через math/big.
	uBig := new(big.Int)
	for i := len(uSmall) - 1; i >= 0; i-- {
		uBig.Lsh(uBig, 64)
		uBig.Or(uBig, new(big.Int).SetUint64(uSmall[i]))
	}
	dBig := new(big.Int).SetUint64(d1.Uint64())
	_, rBig := new(big.Int).QuoRem(uBig, dBig, new(big.Int))

	// rem должен иметь только младший слова, остальные — нули.
	if rem[1] != 0 || rem[2] != 0 || rem[3] != 0 {
		t.Fatalf("rem high words not cleared: rem=%v (expected only [0] non-zero)", rem)
	}
	if rem[0] != rBig.Uint64() {
		t.Fatalf("rem[0] mismatch: got %d, want %d", rem[0], rBig.Uint64())
	}
	if rem.IsZero() {
		// not zero in this case but the check is to ensure IsZero would respect cleared words
		t.Fatalf("rem should not be zero; if this fires, the test data is bad")
	}
}

// TestUdivremCacheConsistency — verifies that repeated divisions with the
// SAME divisor produce the same result, and that switching divisors back and
// forth doesn't corrupt the cache (lastDn/lastShift/lastDLen).
func TestUdivremCacheConsistency(t *testing.T) {
	ut := NewUint256Utils()
	rng := rand.New(rand.NewSource(0xCACECACE))

	divisors := []*uint256.Int{
		uint256.NewInt(1_000_000),                                                                               // 1-word, fee
		uint256.MustFromHex("0xde0b6b3a7640000"),                                                                // 1-word liquidity
		uint256.MustFromHex("0x100000000000000000000"),                                                          // 2-word
		uint256.MustFromHex("0xffffffffffffffffffffffffffffffff"),                                               // 2-word max
		uint256.MustFromHex("0x100000000000000000000000000000000000000"),                                        // 3-word sqrt-like
		uint256.MustFromHex("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),                       // 3-word large
		uint256.MustFromHex("0x100000000000000000000000000000000000000000000000000000000000001"),                // 4-word
		uint256.MustFromHex("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),               // 4-word max-ish
	}

	const trials = 200
	for trial := 0; trial < trials; trial++ {
		d := divisors[rng.Intn(len(divisors))]
		u := make([]uint64, 8)
		for i := range u {
			u[i] = rng.Uint64()
		}

		// Reference via math/big.
		uBig := new(big.Int)
		for i := len(u) - 1; i >= 0; i-- {
			uBig.Lsh(uBig, 64)
			uBig.Or(uBig, new(big.Int).SetUint64(u[i]))
		}
		dBig := d.ToBig()
		qBig, rBig := new(big.Int).QuoRem(uBig, dBig, new(big.Int))

		quot := make([]uint64, 8)
		rem := new(uint256.Int)
		ut.udivrem(quot, u, d, rem)

		gotQ := new(big.Int)
		for i := len(quot) - 1; i >= 0; i-- {
			gotQ.Lsh(gotQ, 64)
			gotQ.Or(gotQ, new(big.Int).SetUint64(quot[i]))
		}
		gotR := rem.ToBig()

		if gotQ.Cmp(qBig) != 0 || gotR.Cmp(rBig) != 0 {
			t.Fatalf("trial %d: divisor=%s u=%v\n  truth: q=%s r=%s\n  got:   q=%s r=%s",
				trial, d.Hex(), u, qBig.Text(16), rBig.Text(16), gotQ.Text(16), gotR.Text(16))
		}
	}
}

func randUintForRoundtrip(rng *rand.Rand) *uint256.Int {
	var x uint256.Int
	x[0] = rng.Uint64()
	x[1] = rng.Uint64()
	x[2] = rng.Uint64()
	x[3] = rng.Uint64()
	switch rng.Intn(4) {
	case 0:
		x[2], x[3] = 0, 0
	case 1:
		x[3] = 0
	}
	return &x
}
