// BSD 3-Clause License
//
// Copyright 2020 uint256 Authors
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//   list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
//   this list of conditions and the following disclaimer in the documentation
//   and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its
//   contributors may be used to endorse or promote products derived from
//   this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package utils

import (
	"math/bits"

	"github.com/holiman/uint256"
)

type Uint256Utils struct {
	dnStorage [5]uint64
	unStorage [9]uint64

	// Кэш для last-seen делителя.
	// Внутри Pool.Swap большинство делений идут с одним и тем же делителем
	// (pool.Liquidity, sqrtPriceX96), и hardware DIV в reciprocal2by1 (~30 циклов)
	// пропускается на cache-hit. На профиле udivremKnuth2/By1 ≈ 21% CPU,
	// большая часть из них — повторы одного и того же делителя.
	//
	// Кроме reciprocal-а кэшируем нормализованный делитель, dLen и shift —
	// тогда на hit пропускается весь switch+нормализация (~6 ns/call), и
	// внутри dnStorage уже лежат правильные значения от прошлого вызова.
	lastD0, lastD1, lastD2, lastD3     uint64 // исходный делитель (для сверки)
	lastDn0, lastDn1, lastDn2, lastDn3 uint64 // нормализованный делитель (копия dnStorage)
	lastRecip                          uint64 // reciprocal2by1(нормализованного старшего слова)
	lastShift                          uint   // bits.LeadingZeros64(d[dLen-1])
	lastDLen                           int    // 1..4
	hasLastD                           bool
}

func NewUint256Utils() *Uint256Utils {
	return &Uint256Utils{}
}


// umul computes full 256 x 256 -> 512 multiplication.
func umul(x, y *uint256.Int) [8]uint64 {
	var (
		res                           [8]uint64
		carry, carry4, carry5, carry6 uint64
		res1, res2, res3, res4, res5  uint64
	)

	carry, res[0] = bits.Mul64(x[0], y[0])
	carry, res1 = umulHop(carry, x[1], y[0])
	carry, res2 = umulHop(carry, x[2], y[0])
	carry4, res3 = umulHop(carry, x[3], y[0])

	carry, res[1] = umulHop(res1, x[0], y[1])
	carry, res2 = umulStep(res2, x[1], y[1], carry)
	carry, res3 = umulStep(res3, x[2], y[1], carry)
	carry5, res4 = umulStep(carry4, x[3], y[1], carry)

	carry, res[2] = umulHop(res2, x[0], y[2])
	carry, res3 = umulStep(res3, x[1], y[2], carry)
	carry, res4 = umulStep(res4, x[2], y[2], carry)
	carry6, res5 = umulStep(carry5, x[3], y[2], carry)

	carry, res[3] = umulHop(res3, x[0], y[3])
	carry, res[4] = umulStep(res4, x[1], y[3], carry)
	carry, res[5] = umulStep(res5, x[2], y[3], carry)
	res[7], res[6] = umulStep(carry6, x[3], y[3], carry)

	return res
}

// umulHop computes (hi * 2^64 + lo) = z + (x * y)
func umulHop(z, x, y uint64) (hi, lo uint64) {
	hi, lo = bits.Mul64(x, y)
	lo, carry := bits.Add64(lo, z, 0)
	hi, _ = bits.Add64(hi, 0, carry)
	return hi, lo
}

// umulStep computes (hi * 2^64 + lo) = z + (x * y) + carry.
func umulStep(z, x, y, carry uint64) (hi, lo uint64) {
	hi, lo = bits.Mul64(x, y)
	lo, carry = bits.Add64(lo, carry, 0)
	hi, _ = bits.Add64(hi, 0, carry)
	lo, carry = bits.Add64(lo, z, 0)
	hi, _ = bits.Add64(hi, 0, carry)
	return hi, lo
}

// udivrem divides u by d and produces both quotient and remainder.
// The quotient is stored in provided quot - len(u)-len(d)+1 words.
// It loosely follows the Knuth's division algorithm (sometimes referenced as "schoolbook" division) using 64-bit words.
// See Knuth, Volume 2, section 4.3.1, Algorithm D.
func (ut *Uint256Utils) udivremV1(quot, u []uint64, d *uint256.Int, rem *uint256.Int) {
	var dLen int
	for i := len(d) - 1; i >= 0; i-- {
		if d[i] != 0 {
			dLen = i + 1
			break
		}
	}

	shift := bits.LeadingZeros64(d[dLen-1])

	// ut.dnStorage.Clear()
	dn := ut.dnStorage[:dLen]
	for i := dLen - 1; i > 0; i-- {
		dn[i] = (d[i] << shift) | (d[i-1] >> (64 - shift))
	}
	dn[0] = d[0] << shift

	var uLen int
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] != 0 {
			uLen = i + 1
			break
		}
	}

	if uLen < dLen {
		copy(rem[:], u)
		return
	}

	// var unStorage [9]uint64
	un := ut.unStorage[:uLen+1]

	un[uLen] = u[uLen-1] >> (64 - shift)
	for i := uLen - 1; i > 0; i-- {
		un[i] = (u[i] << shift) | (u[i-1] >> (64 - shift))
	}
	un[0] = u[0] << shift

	// Skip the highest word of numerator if not significant (saves one word in udivremBy1 only).
	// For dLen==1 safe when un[uLen]==0 && un[uLen-1]<dn[0] (top quotient digit is 0). For dLen>1 cannot skip: Knuth first iteration modifies u for the next.
	if dLen == 1 && un[uLen] == 0 && un[uLen-1] < dn[0] && uLen >= 2 {
		un = un[:uLen]
		rem.SetUint64(udivremBy1(quot, un, dn[0]) >> shift)
		quot[uLen-1] = 0
		return
	}
	if dLen == 1 {
		rem.SetUint64(udivremBy1(quot, un, dn[0]) >> shift)
		return
	}
	udivremKnuth(quot, un, dn)

	for i := 0; i < dLen-1; i++ {
		rem[i] = (un[i] >> shift) | (un[i+1] << (64 - shift))
	}
	rem[dLen-1] = un[dLen-1] >> shift
}

// udivrem divides u by d and produces both quotient and remainder.
// The quotient is stored in provided quot - len(u)-len(d)+1 words.
// It loosely follows the Knuth's division algorithm (sometimes referenced as "schoolbook" division) using 64-bit words.
// See Knuth, Volume 2, section 4.3.1, Algorithm D.
func (ut *Uint256Utils) udivrem(quot, u []uint64, d *uint256.Int, rem *uint256.Int) {
	var dLen int
	var shift uint
	dn := ut.dnStorage[:4]

	// Cache hit: full state of normalised divisor доступен от прошлого вызова.
	// Скипаем switch+normalization (~6 ns) — критично т.к. udivrem на профиле = 10.4% CPU.
	if ut.hasLastD &&
		d[0] == ut.lastD0 && d[1] == ut.lastD1 &&
		d[2] == ut.lastD2 && d[3] == ut.lastD3 {
		dLen = ut.lastDLen
		shift = ut.lastShift
		dn[0] = ut.lastDn0
		dn[1] = ut.lastDn1
		dn[2] = ut.lastDn2
		dn[3] = ut.lastDn3
	} else {
		switch {
		case d[3] != 0:
			dLen = 4
			shift = uint(bits.LeadingZeros64(d[3]))
			dn[3] = (d[3] << shift) | (d[2] >> (64 - shift))
			dn[2] = (d[2] << shift) | (d[1] >> (64 - shift))
			dn[1] = (d[1] << shift) | (d[0] >> (64 - shift))
			dn[0] = d[0] << shift
		case d[2] != 0:
			dLen = 3
			shift = uint(bits.LeadingZeros64(d[2]))
			dn[2] = (d[2] << shift) | (d[1] >> (64 - shift))
			dn[1] = (d[1] << shift) | (d[0] >> (64 - shift))
			dn[0] = d[0] << shift
		case d[1] != 0:
			dLen = 2
			shift = uint(bits.LeadingZeros64(d[1]))
			dn[1] = (d[1] << shift) | (d[0] >> (64 - shift))
			dn[0] = d[0] << shift
		case d[0] != 0:
			dLen = 1
			shift = uint(bits.LeadingZeros64(d[0]))
			dn[0] = d[0] << shift
		default:
			dLen = 0
			// обработка ошибки — деление на 0, например panic
		}
		recip := reciprocal2by1(dn[dLen-1])
		ut.lastD0, ut.lastD1, ut.lastD2, ut.lastD3 = d[0], d[1], d[2], d[3]
		ut.lastDn0, ut.lastDn1, ut.lastDn2, ut.lastDn3 = dn[0], dn[1], dn[2], dn[3]
		ut.lastShift = shift
		ut.lastDLen = dLen
		ut.lastRecip = recip
		ut.hasLastD = true
	}
	dn = dn[:dLen]
	recip := ut.lastRecip

	var uLen int
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] != 0 {
			uLen = i + 1
			break
		}
	}

	if uLen < dLen {
		// Полное обновление rem: copy(u) пишет min(len(rem), len(u)) слов.
		// Старшие слова rem могут содержать stale-данные от прошлого вызова с
		// большим dLen, поэтому явно очищаем их (см. fix-up в обычной ветке ниже).
		rem[0], rem[1], rem[2], rem[3] = 0, 0, 0, 0
		copy(rem[:], u)
		return
	}

	// var unStorage [9]uint64
	un := ut.unStorage[:uLen+1]

	un[uLen] = u[uLen-1] >> (64 - shift)
	for i := uLen - 1; i > 0; i-- {
		un[i] = (u[i] << shift) | (u[i-1] >> (64 - shift))
	}
	un[0] = u[0] << shift

	// Skip the highest word of numerator if not significant (saves one word in udivremBy1 only).
	// For dLen==1 safe when un[uLen]==0 && un[uLen-1]<dn[0] (top quotient digit is 0). For dLen>1 cannot skip: Knuth first iteration modifies u for the next.
	if dLen == 1 && un[uLen] == 0 && un[uLen-1] < dn[0] && uLen >= 2 {
		un = un[:uLen]
		rem.SetUint64(udivremBy1WithRecip(quot, un, dn[0], recip) >> shift)
		quot[uLen-1] = 0
		return
	}
	if dLen == 1 {
		rem.SetUint64(udivremBy1WithRecip(quot, un, dn[0], recip) >> shift)
		return
	}
	switch dLen {
	case 2:
		udivremKnuth2WithRecip(quot, un, dn[0], dn[1], recip)
	case 3:
		udivremKnuth3WithRecip(quot, un, dn[0], dn[1], dn[2], recip)
	default:
		udivremKnuthWithRecip(quot, un, dn, recip)
	}

	// FIX: ниже dLen<4 ветки писали только rem[0..dLen-1], старшие слова оставались
	// stale от прошлого вызова (если он был с большим dLen). При следующем
	// IsZero()/Eq() это приводило к неверному ответу. Сейчас всегда пишем все 4 слова.
	switch dLen {
	case 4:
		rem[0] = (un[0] >> shift) | (un[1] << (64 - shift))
		rem[1] = (un[1] >> shift) | (un[2] << (64 - shift))
		rem[2] = (un[2] >> shift) | (un[3] << (64 - shift))
		rem[3] = un[3] >> shift
	case 3:
		rem[0] = (un[0] >> shift) | (un[1] << (64 - shift))
		rem[1] = (un[1] >> shift) | (un[2] << (64 - shift))
		rem[2] = un[2] >> shift
		rem[3] = 0
	case 2:
		rem[0] = (un[0] >> shift) | (un[1] << (64 - shift))
		rem[1] = un[1] >> shift
		rem[2] = 0
		rem[3] = 0
	case 1:
		rem[0] = un[0] >> shift
		rem[1] = 0
		rem[2] = 0
		rem[3] = 0
	}
}

// udivremBy1 divides u by single normalized word d and produces both quotient and remainder.
// The quotient is stored in provided quot.
func udivremBy1(quot, u []uint64, d uint64) (rem uint64) {
	return udivremBy1WithRecip(quot, u, d, reciprocal2by1(d))
}

// udivremBy1WithRecip — pure-Go реализация деления многословного u на одно
// нормализованное слово d с предвычисленным reciprocal. Тело — цикл по словам
// числителя в обратном порядке через udivrem2by1 (Möller–Granlund Algorithm 4).
//
// EXPERIMENT (см. историю изменений / коммит-сообщение):
// Ручная AMD64 ASM реализация (MULXQ + ветвистая коррекция) измерялась против
// этой версии в end-to-end симуляторе и оказалась ~3% МЕДЛЕННЕЕ. Причина:
//  1. С GOAMD64=v4 Go компилятор уже генерит почти оптимальный код:
//     MULQ + ADDQ/ADCQ + LEA + CMOVB/CMOVBE (branchless коррекция),
//     что лучше предсказуемо в hot-loop, чем условные переходы.
//  2. Замена на function-variable dispatch (`var udivrem... = ...`) ломает
//     компилятору возможность инлайнить эту функцию в (*Uint256Utils).udivrem,
//     добавляя cost ~1-2 ns indirect call на каждый вызов.
// Поэтому сейчас держим pure-Go как единственную реализацию.
func udivremBy1WithRecip(quot, u []uint64, d, reciprocal uint64) (rem uint64) {
	lenU := len(u)
	rem = u[lenU-1] // Set the top word as remainder.
	for j := lenU - 2; j >= 0; j-- {
		quot[j], rem = udivrem2by1(rem, u[j], d, reciprocal)
	}
	return rem
}

// reciprocal2by1 computes <^d, ^0> / d.
func reciprocal2by1(d uint64) uint64 {
	reciprocal, _ := bits.Div64(^d, ^uint64(0), d)
	return reciprocal
}

// udivrem2by1 divides <uh, ul> / d and produces both quotient and remainder.
// It uses the provided d's reciprocal.
// Implementation ported from https://github.com/chfast/intx and is based on
// "Improved division by invariant integers", Algorithm 4 (Möller, Granlund 2011).
//
// Bug-fix note: the original port had `if r >= d` NESTED inside `if r > ql`,
// which is incorrect. After step `qh++` the estimate qh may still be 1 too low
// (because v=floor((B^2-1)/d) - B is itself rounded down), in which case
// r >= d but no underflow occurred (so r ≤ ql). The second check must be a
// SEPARATE top-level conditional, exactly as in chfast/intx C++ reference.
// A 60-second fuzz against math/big surfaces this within ~1ms once the random
// space is broad enough; the previous test corpus happened to miss it.
func udivrem2by1(uh, ul, d, reciprocal uint64) (quot, rem uint64) {
	qh, ql := bits.Mul64(reciprocal, uh)
	ql, carry := bits.Add64(ql, ul, 0)
	qh, _ = bits.Add64(qh, uh, carry)
	qh++

	r := ul - qh*d

	if r > ql {
		qh--
		r += d
	}
	if r >= d {
		qh++
		r -= d
	}

	return qh, r
}

// udivremKnuth implements the division of u by normalized multiple word d from the Knuth's division algorithm.
// The quotient is stored in provided quot - len(u)-len(d) words.
// Updates u to contain the remainder - len(d) words.
func udivremKnuth(quot, u, d []uint64) {
	udivremKnuthWithRecip(quot, u, d, reciprocal2by1(d[len(d)-1]))
}

// udivremKnuthWithRecip — версия udivremKnuth с предвычисленным reciprocal.
func udivremKnuthWithRecip(quot, u, d []uint64, reciprocal uint64) {
	lenD := len(d)
	dh := d[lenD-1]
	dl := d[lenD-2]

	for j := len(u) - lenD - 1; j >= 0; j-- {
		u2 := u[j+lenD]
		u1 := u[j+lenD-1]
		u0 := u[j+lenD-2]

		var qhat, rhat uint64
		if u2 < dh {
			qhat, rhat = udivrem2by1(u2, u1, dh, reciprocal)
			ph, pl := bits.Mul64(qhat, dl)
			if ph > rhat || (ph == rhat && pl > u0) {
				qhat--
			}
		} else {
			qhat = ^uint64(0)
		}

		// Multiply and subtract.
		borrow := subMulTo(u[j:], d, qhat)

		ujd := &u[j+lenD]
		old := *ujd
		*ujd = old - borrow
		if old < borrow {
			qhat--
			*ujd += addTo(u[j:], d)
		}

		quot[j] = qhat
	}
}

// udivremKnuth3 — специализация udivremKnuth для dLen=3.
// Инлайнит subMulTo(case 3) и addTo(case 3), устраняя оверхед вызовов и switch-диспетчеризации.
func udivremKnuth3(quot, u []uint64, d0, d1, d2 uint64) {
	udivremKnuth3WithRecip(quot, u, d0, d1, d2, reciprocal2by1(d2))
}

// udivremKnuth3WithRecip — версия udivremKnuth3 с предвычисленным reciprocal.
func udivremKnuth3WithRecip(quot, u []uint64, d0, d1, d2, reciprocal uint64) {

	for j := len(u) - 4; j >= 0; j-- {
		u2 := u[j+3]
		u1 := u[j+2]
		u0 := u[j+1]

		var qhat uint64
		if u2 < d2 {
			var rhat uint64
			qhat, rhat = udivrem2by1(u2, u1, d2, reciprocal)
			ph, pl := bits.Mul64(qhat, d1)
			if ph > rhat || (ph == rhat && pl > u0) {
				qhat--
			}
		} else {
			qhat = ^uint64(0)
		}

		// inlined subMulTo for 3 words
		ph, pl := bits.Mul64(d0, qhat)
		t, carry2 := bits.Sub64(u[j], pl, 0)
		u[j] = t
		borrow := ph + carry2

		s, carry1 := bits.Sub64(u[j+1], borrow, 0)
		ph, pl = bits.Mul64(d1, qhat)
		t, carry2 = bits.Sub64(s, pl, 0)
		u[j+1] = t
		borrow = ph + carry1 + carry2

		s, carry1 = bits.Sub64(u[j+2], borrow, 0)
		ph, pl = bits.Mul64(d2, qhat)
		t, carry2 = bits.Sub64(s, pl, 0)
		u[j+2] = t
		borrow = ph + carry1 + carry2

		old := u[j+3]
		u[j+3] = old - borrow
		if old < borrow {
			qhat--
			var c uint64
			u[j], c = bits.Add64(u[j], d0, 0)
			u[j+1], c = bits.Add64(u[j+1], d1, c)
			u[j+2], c = bits.Add64(u[j+2], d2, c)
			u[j+3] += c
		}

		quot[j] = qhat
	}
}

// udivremKnuth2 — специализация udivremKnuth для dLen=2.
func udivremKnuth2(quot, u []uint64, d0, d1 uint64) {
	udivremKnuth2WithRecip(quot, u, d0, d1, reciprocal2by1(d1))
}

// udivremKnuth2WithRecip — версия udivremKnuth2 с предвычисленным reciprocal.
func udivremKnuth2WithRecip(quot, u []uint64, d0, d1, reciprocal uint64) {

	for j := len(u) - 3; j >= 0; j-- {
		u2 := u[j+2]
		u1 := u[j+1]
		u0 := u[j]

		var qhat uint64
		if u2 < d1 {
			var rhat uint64
			qhat, rhat = udivrem2by1(u2, u1, d1, reciprocal)
			ph, pl := bits.Mul64(qhat, d0)
			if ph > rhat || (ph == rhat && pl > u0) {
				qhat--
			}
		} else {
			qhat = ^uint64(0)
		}

		// inlined subMulTo for 2 words
		ph, pl := bits.Mul64(d0, qhat)
		t, carry2 := bits.Sub64(u[j], pl, 0)
		u[j] = t
		borrow := ph + carry2

		s, carry1 := bits.Sub64(u[j+1], borrow, 0)
		ph, pl = bits.Mul64(d1, qhat)
		t, carry2 = bits.Sub64(s, pl, 0)
		u[j+1] = t
		borrow = ph + carry1 + carry2

		old := u[j+2]
		u[j+2] = old - borrow
		if old < borrow {
			qhat--
			var c uint64
			u[j], c = bits.Add64(u[j], d0, 0)
			u[j+1], c = bits.Add64(u[j+1], d1, c)
			u[j+2] += c
		}

		quot[j] = qhat
	}
}

// subMulTo computes x -= y * multiplier.
// Requires len(x) >= len(y).
func subMulTo(x, y []uint64, multiplier uint64) uint64 {
	var borrow uint64
	switch len(y) {
	// case 0:
	// 	return 0

	// case 1:
	// 	s, carry1 := bits.Sub64(x[0], borrow, 0)
	// 	ph, pl := bits.Mul64(y[0], multiplier)
	// 	t, carry2 := bits.Sub64(s, pl, 0)
	// 	x[0] = t
	// 	borrow = ph + carry1 + carry2

	case 2:
		// borrow == 0 at entry: skip Sub64(x[0], 0, 0)
		ph, pl := bits.Mul64(y[0], multiplier)
		t, carry2 := bits.Sub64(x[0], pl, 0)
		x[0] = t
		borrow = ph + carry2

		s, carry1 := bits.Sub64(x[1], borrow, 0)
		ph, pl = bits.Mul64(y[1], multiplier)
		t, carry2 = bits.Sub64(s, pl, 0)
		x[1] = t
		borrow = ph + carry1 + carry2

	case 3:
		// borrow == 0 at entry: skip Sub64(x[0], 0, 0)
		ph, pl := bits.Mul64(y[0], multiplier)
		t, carry2 := bits.Sub64(x[0], pl, 0)
		x[0] = t
		borrow = ph + carry2

		s, carry1 := bits.Sub64(x[1], borrow, 0)
		ph, pl = bits.Mul64(y[1], multiplier)
		t, carry2 = bits.Sub64(s, pl, 0)
		x[1] = t
		borrow = ph + carry1 + carry2

		s, carry1 = bits.Sub64(x[2], borrow, 0)
		ph, pl = bits.Mul64(y[2], multiplier)
		t, carry2 = bits.Sub64(s, pl, 0)
		x[2] = t
		borrow = ph + carry1 + carry2

	case 4:
		// borrow == 0 at entry: skip Sub64(x[0], 0, 0)
		ph, pl := bits.Mul64(y[0], multiplier)
		t, carry2 := bits.Sub64(x[0], pl, 0)
		x[0] = t
		borrow = ph + carry2

		s, carry1 := bits.Sub64(x[1], borrow, 0)
		ph, pl = bits.Mul64(y[1], multiplier)
		t, carry2 = bits.Sub64(s, pl, 0)
		x[1] = t
		borrow = ph + carry1 + carry2

		s, carry1 = bits.Sub64(x[2], borrow, 0)
		ph, pl = bits.Mul64(y[2], multiplier)
		t, carry2 = bits.Sub64(s, pl, 0)
		x[2] = t
		borrow = ph + carry1 + carry2

		s, carry1 = bits.Sub64(x[3], borrow, 0)
		ph, pl = bits.Mul64(y[3], multiplier)
		t, carry2 = bits.Sub64(s, pl, 0)
		x[3] = t
		borrow = ph + carry1 + carry2

	default:
		// по факту сюда не попадаем и не должны
		for i := 0; i < len(y); i++ {
			s, carry1 := bits.Sub64(x[i], borrow, 0)
			ph, pl := bits.Mul64(y[i], multiplier)
			t, carry2 := bits.Sub64(s, pl, 0)
			x[i] = t
			borrow = ph + carry1 + carry2
		}
	}
	return borrow
}

// umul_lo3 вычисляет полное 256×256→512-битное произведение для частного случая:
// x[0]=0 (числитель = liquidity<<96) и y[3]=0 (160-битный операнд).
// Выполняет 9 Mul64 вместо 16 (пропускает нулевые строку x[0] и столбец y[3]).
func umul_lo3(x, y *uint256.Int) [8]uint64 {
	var (
		res                           [8]uint64
		carry, carry4, carry5, carry6 uint64
		res2, res3, res4, res5        uint64
	)
	// Строка y[0]: x[0]=0 → p[0]=0; начинаем с x[1]
	var res1 uint64
	carry, res1 = bits.Mul64(x[1], y[0])
	carry, res2 = umulHop(carry, x[2], y[0])
	carry4, res3 = umulHop(carry, x[3], y[0])

	// Строка y[1]: x[0]*y[1]=0 → res[1]=res1
	res[1] = res1
	carry, res2 = umulHop(res2, x[1], y[1])
	carry, res3 = umulStep(res3, x[2], y[1], carry)
	carry5, res4 = umulStep(carry4, x[3], y[1], carry)

	// Строка y[2]: x[0]*y[2]=0 → res[2]=res2
	res[2] = res2
	carry, res3 = umulHop(res3, x[1], y[2])
	carry, res4 = umulStep(res4, x[2], y[2], carry)
	carry6, res5 = umulStep(carry5, x[3], y[2], carry)

	// Строка y[3]=0: все слагаемые нулевые
	res[3] = res3
	res[4] = res4
	res[5] = res5
	res[6] = carry6
	// res[7] = 0

	return res
}

// mulRsh96_2x3 computes floor(a*b / 2^96) into result and returns whether (a*b % 2^96) != 0.
//
// Требования: a ≤ 2^128 (a[2]=a[3]=0), b ≤ 2^160 (b[3]=0).
// Выполняет 6 ops Mul64 вместо 16 для общего umul и полностью избегает деления Кнута:
// для знаменателя 2^96 деление — это просто сдвиг вправо на 96 бит.
func mulRsh96_2x3(a, b, result *uint256.Int) bool {
	// 6 частичных произведений:
	h00, l00 := bits.Mul64(a[0], b[0]) //   → биты   0-127
	h01, l01 := bits.Mul64(a[0], b[1]) //   → биты  64-191
	h02, l02 := bits.Mul64(a[0], b[2]) //   → биты 128-255
	h10, l10 := bits.Mul64(a[1], b[0]) //   → биты  64-191
	h11, l11 := bits.Mul64(a[1], b[1]) //   → биты 128-255
	h12, l12 := bits.Mul64(a[1], b[2]) //   → биты 192-319

	// p[1]: h00 + l01 + l10  (биты 64-127 произведения)
	var c0, c1 uint64
	p1, c0 := bits.Add64(h00, l01, 0)
	p1, c1 = bits.Add64(p1, l10, 0)

	// p[2]: c0+c1 + h01 + h10 + l02 + l11  (биты 128-191)
	var c2, c3, c4, c5, c6 uint64
	p2, c2 := bits.Add64(h01, h10, 0)
	p2, c3 = bits.Add64(p2, l02, 0)
	p2, c4 = bits.Add64(p2, l11, 0)
	p2, c5 = bits.Add64(p2, c0, 0)
	p2, c6 = bits.Add64(p2, c1, 0)
	carry2 := c2 + c3 + c4 + c5 + c6 // ≤ 5

	// p[3]: carry2 + h02 + h11 + l12  (биты 192-255)
	var c7, c8, c9 uint64
	p3, c7 := bits.Add64(h02, h11, 0)
	p3, c8 = bits.Add64(p3, l12, 0)
	p3, c9 = bits.Add64(p3, carry2, 0)
	carry3 := c7 + c8 + c9 // ≤ 3

	// p[4]: carry3 + h12  (биты 256-287; ≤ 2^32-1 для a≤2^128, b≤2^160)
	p4 := h12 + carry3

	// Сдвиг вправо на 96 бит = вычитаем 1 полное слово (64 бита) + ещё 32 бита:
	//   result[i] = (p[i+1] >> 32) | (p[i+2] << 32)
	result[0] = (p1 >> 32) | (p2 << 32)
	result[1] = (p2 >> 32) | (p3 << 32)
	result[2] = (p3 >> 32) | (p4 << 32)
	result[3] = p4 >> 32 // должен быть 0 при корректных входных данных

	// Остаток ненулевой ↔ хотя бы один из младших 96 бит произведения ненулевой:
	// биты 0-63 = l00, биты 64-95 = нижние 32 бита p1.
	return l00 != 0 || (p1&0xFFFFFFFF) != 0
}

// Предвычисленные константы для деления на MaxFee = 1_000_000 (20-битная константа).
// Нормализованный делитель (сдвиг на 44 бита) и его реципрокал позволяют заменить
// hardware DIV (~30 цикл.) в reciprocal2by1 на 5 умножений bits.Mul64 (~5 цикл.) за вызов.
var (
	maxFeeShift = uint(bits.LeadingZeros64(1_000_000)) // = 44
	maxFeeNorm  = uint64(1_000_000) << uint(bits.LeadingZeros64(1_000_000))
	maxFeeRecip = func() uint64 {
		r, _ := bits.Div64(^(uint64(1_000_000) << uint(bits.LeadingZeros64(1_000_000))),
			^uint64(0),
			uint64(1_000_000)<<uint(bits.LeadingZeros64(1_000_000)))
		return r
	}()
)

// DivByMaxFeeInto — экспортированная обёртка над divByMaxFeeInto: позволяет внешним
// потребителям (например, defisimulator/InlineFeeProcessor) выполнять деление на
// MaxFee=1_000_000 быстрее, чем holiman.Div (≈ 30 ns vs ≈ 60 ns на профиле).
//
//go:nosplit
func DivByMaxFeeInto(a, result *uint256.Int) {
	divByMaxFeeInto(a, result)
}

// divByMaxFeeInto вычисляет result = floor(a / 1_000_000) без вычисления реципрокала.
// Использует предвычисленные maxFeeNorm и maxFeeRecip: заменяет hardware DIV (~30 цикл.)
// в reciprocal2by1 на 5 умножений bits.Mul64 (~5 цикл.).
// Адаптирует число итераций под реальный размер a (1-4 слова) — так же, как udivremBy1.
func divByMaxFeeInto(a, result *uint256.Int) {
	shift := maxFeeShift
	rshift := 64 - shift
	d, recip := maxFeeNorm, maxFeeRecip
	var rem uint64

	switch {
	case a[3] != 0:
		// 4-слова: uLen=4, нормализованный дивиденд имеет 5 слов
		un4 := a[3] >> rshift
		un3 := (a[3] << shift) | (a[2] >> rshift)
		un2 := (a[2] << shift) | (a[1] >> rshift)
		un1 := (a[1] << shift) | (a[0] >> rshift)
		un0 := a[0] << shift
		rem = un4
		var q3, q2, q1, q0 uint64
		q3, rem = udivrem2by1(rem, un3, d, recip)
		q2, rem = udivrem2by1(rem, un2, d, recip)
		q1, rem = udivrem2by1(rem, un1, d, recip)
		q0, rem = udivrem2by1(rem, un0, d, recip)
		result[0], result[1], result[2], result[3] = q0, q1, q2, q3

	case a[2] != 0:
		// 3-слова: uLen=3, 4 нормализованных слова
		un3 := a[2] >> rshift
		un2 := (a[2] << shift) | (a[1] >> rshift)
		un1 := (a[1] << shift) | (a[0] >> rshift)
		un0 := a[0] << shift
		rem = un3
		var q2, q1, q0 uint64
		q2, rem = udivrem2by1(rem, un2, d, recip)
		q1, rem = udivrem2by1(rem, un1, d, recip)
		q0, rem = udivrem2by1(rem, un0, d, recip)
		result[0], result[1], result[2], result[3] = q0, q1, q2, 0

	case a[1] != 0:
		// 2-слова: uLen=2, 3 нормализованных слова
		un2 := a[1] >> rshift
		un1 := (a[1] << shift) | (a[0] >> rshift)
		un0 := a[0] << shift
		rem = un2
		var q1, q0 uint64
		q1, rem = udivrem2by1(rem, un1, d, recip)
		q0, rem = udivrem2by1(rem, un0, d, recip)
		result[0], result[1], result[2], result[3] = q0, q1, 0, 0

	default:
		// 1-слово: uLen=1, 2 нормализованных слова
		un1 := a[0] >> rshift
		un0 := a[0] << shift
		rem = un1
		var q0 uint64
		q0, rem = udivrem2by1(rem, un0, d, recip)
		result[0], result[1], result[2], result[3] = q0, 0, 0, 0
	}
	_ = rem // remainder не нужен
}

// addTo computes x += y.
// Requires len(x) >= len(y).
func addTo(x, y []uint64) uint64 {
	var carry uint64
	switch len(y) {
	// case 0:
	// 	return 0
	// case 1:
	// 	x[0], carry = bits.Add64(x[0], y[0], 0)
	case 2:
		x[0], carry = bits.Add64(x[0], y[0], 0)
		x[1], carry = bits.Add64(x[1], y[1], carry)
	case 3:
		x[0], carry = bits.Add64(x[0], y[0], 0)
		x[1], carry = bits.Add64(x[1], y[1], carry)
		x[2], carry = bits.Add64(x[2], y[2], carry)
	case 4:
		x[0], carry = bits.Add64(x[0], y[0], 0)
		x[1], carry = bits.Add64(x[1], y[1], carry)
		x[2], carry = bits.Add64(x[2], y[2], carry)
		x[3], carry = bits.Add64(x[3], y[3], carry)
	default:
		// по факту сюда не попадаем и не должны
		for i := 0; i < len(y); i++ {
			x[i], carry = bits.Add64(x[i], y[i], carry)
		}
	}
	return carry
}
