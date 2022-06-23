package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	nanosMin = -999999999
	nanosMax = +999999999
	nanosMod = 1000000000
)

type Money struct {
	Units        int64
	Nanos        int32
	CurrencyCode string
}

func (x *Money) GetCurrencyCode() string {
	if x != nil {
		return x.CurrencyCode
	}
	return ""
}

func (x *Money) GetUnits() int64 {
	if x != nil {
		return x.Units
	}
	return 0
}

func (x *Money) GetNanos() int32 {
	if x != nil {
		return x.Nanos
	}
	return 0
}

var (
	// ErrInvalidMultiplierProvided is returned when a negative or zero multiplier is provided.
	ErrInvalidMultiplierProvided = errors.New("multiplier provided is zero or negative which is invalid")

	// ErrInvalidValue is returned if the specified money amount is not valid.
	ErrInvalidValue = errors.New("one of the specified money values is invalid")

	// ErrMismatchingCurrency is returned if two values don't have the same currency code.
	ErrMismatchingCurrency = errors.New("mismatching currency codes")
)

/*
This package will contain all the conversions from and to google.Money
reference: https://github.com/googleapis/googleapis/blob/master/google/type/money.proto
*/

// IsGreaterThan if a>b return true, else return false
func IsGreaterThan(a, b *Money) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	if a.Units > b.Units {
		return true
	}
	if a.Units < b.Units {
		return false
	}
	return a.Nanos > b.Nanos
}

// fromInt will convert integer to google.Money
func fromInt(amount, currencyMultiplier int64, currencyCode string) *Money {
	if amount == 0 {
		return &Money{
			CurrencyCode: currencyCode,
			Units:        0,
			Nanos:        0,
		}
	}

	nanos := float64(amount % currencyMultiplier)
	nanosAdjusted := nanos * (math.Pow10(9) / float64(currencyMultiplier))
	units := amount / currencyMultiplier

	return &Money{
		CurrencyCode: currencyCode,
		Units:        units,
		Nanos:        int32(nanosAdjusted),
	}
}

// asInt will convert google.Money to integer
func asInt(money *Money, currencyMultiplier int64) int64 {
	if money == nil {
		return 0
	}

	var nanosAdjusted int64

	if money.Nanos == 0 {
		nanosAdjusted = 0
	} else {
		nanosAdjustedFloat := float64(money.Nanos) * math.Pow10(-9) * float64(currencyMultiplier)
		nanosAdjusted = int64(nanosAdjustedFloat)
	}

	return money.Units*currencyMultiplier + nanosAdjusted
}

// numDecPlaces returns the amount of decimals digits
func numDecPlaces(v float64) int32 {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i > -1 {
		return int32(len(s) - i - 1)
	}

	return 0
}

// FromInt64 will convert int64 value to google.Money ty
func FromInt64(amount, currencyMultiplier int64, currencyCode string) *Money {
	return fromInt(amount, currencyMultiplier, currencyCode)
}

// FromInt32 will convert int64 value to google.Money type
func FromInt32(amount, currencyMultiplier int32, currencyCode string) *Money {
	return fromInt(int64(amount), int64(currencyMultiplier), currencyCode)
}

// AsInt32 will convert google.Money to int32
func AsInt32(money *Money, currencyMultiplier int32) int32 {
	moneyAsInt64 := asInt(money, int64(currencyMultiplier))
	return int32(moneyAsInt64)
}

// AsInt64 will convert google.Money to int64
func AsInt64(money *Money, currencyMultiplier int64) int64 {
	return asInt(money, currencyMultiplier)
}

// IsValid checks if specified value has a valid units/nanos signs and ranges.
func IsValid(m *Money) bool {
	return signMatches(m) && validNanos(m.GetNanos())
}

// signMatches checks if units and nanos signs matches
func signMatches(m *Money) bool {
	return m.GetNanos() == 0 || m.GetUnits() == 0 || (m.GetNanos() < 0) == (m.GetUnits() < 0)
}

// validNanos checks if given nanos are in range
func validNanos(nanos int32) bool { return nanosMin <= nanos && nanos <= nanosMax }

// IsPositive returns true if the specified money value is valid and is
// positive.
func IsPositive(m *Money) bool {
	return IsValid(m) && m.GetUnits() > 0 || (m.GetUnits() == 0 && m.GetNanos() > 0)
}

// IsZero returns true if the specified money value is equal to zero.
func IsZero(m *Money) bool {
	return m.GetUnits() == 0 && m.GetNanos() == 0
}

func Mul(l *Money, r float64) (*Money, error) {
	// It does not make sense to allow multiplication of a price with a negative value as part of the existing flows.
	// We decided because of that to return an error in case a negative value is provided.
	if r < 0 {
		return nil, ErrInvalidMultiplierProvided
	}

	if !IsValid(l) {
		return nil, ErrInvalidValue
	}

	if IsZero(l) || r == float64(0) {
		return &Money{
			CurrencyCode: l.CurrencyCode,
			Units:        0,
			Nanos:        0,
		}, nil
	}

	multiplierDecPlaces := numDecPlaces(r)
	powerOf10 := int32(math.Pow10(int(multiplierDecPlaces)))

	intMulF, decMulF := math.Modf(r)
	intMul, decMul := int64(intMulF), int64(decMulF*float64(powerOf10))

	// multiply both sections
	nanosMultiplied := int64(l.GetNanos()) * intMul
	if decMul != 0 {
		nanosMultiplied += int64(l.GetNanos()) * decMul / int64(powerOf10)
	}

	intUnitsMultiplied := l.GetUnits() * intMul
	decUnitsMultiplied := int64(0)
	if decMul != 0 {
		intUnitsMultiplied += int64(float64(l.GetUnits()*decMul) / float64(powerOf10))
		decUnitsMultiplied = l.GetUnits() * decMul % int64(powerOf10)
	}

	nanosDecUnitAdjusted := decUnitsMultiplied * int64(math.Pow10(9)/float64(powerOf10))

	units := intUnitsMultiplied
	nanos := nanosDecUnitAdjusted + nanosMultiplied

	if (units >= 0 && nanos >= 0) || (units < 0 && nanos <= 0) {
		// same sign <units, nanos>
		units += nanos / nanosMod
		nanos = nanos % nanosMod
	} else {
		// different sign. nanos guaranteed to not to go over the limit
		if units > 0 {
			units--
			nanos += nanosMod
		} else if units < 0 {
			units++
			nanos -= nanosMod
		}
	}

	return &Money{
		Units:        units,
		Nanos:        int32(nanos),
		CurrencyCode: l.GetCurrencyCode()}, nil
}

func test1() {
	l := &Money{Units: 19, Nanos: 13, CurrencyCode: "E"}
	l2, err := Mul(l, 15.11)
	fmt.Println(err, l2)
}

func main() {
	test1()
}
