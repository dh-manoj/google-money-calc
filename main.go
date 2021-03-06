package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
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

func Mulv2(l *Money, r float64) (*Money, error) {
	// fmt.Println("l, r", l, r)
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
	powerOf10 := int64(math.Pow10(int(multiplierDecPlaces)))

	intMulF, decMulF := math.Modf(r)
	intMul, decMul := int64(intMulF), int64(decMulF*float64(powerOf10))

	// To handle edge scenarios where `decMulF*float64(powerOf10)` returns different value than expected.
	// For example: decMulF = 0.29 and powerOf10 = 100 should give 29 rather than 28.
	// Ensure the following invariant is true: decMulF == float64(decMul) / float64(powerOf10)
	// Increment decimal multipler (decMul) if deviation is >= 1%
	newDecMulF := float64(decMul) / float64(powerOf10)
	if newDecMulF < decMulF {
		percentageChange := ((decMulF - newDecMulF) / decMulF) * 100
		if percentageChange >= 0.01 {
			decMul++
		}
	}

	// multiply both sections
	nanosMultiplied := int64(l.GetNanos()) * intMul
	if decMul != 0 {
		// For expressions like: a * b / c, if a and b are of type int64,
		// then a * b could possibly exceed valid int64 range.
		// To handle such scenarios and retain best value,
		// perform division followed by multiplication: a * (b / c)
		nanosMulF := float64(l.GetNanos()) * (float64(decMul) / float64(powerOf10))
		nanosMul := int64(nanosMulF)
		nanosMulDelta := nanosMulF - float64(nanosMul)
		if nanosMulDelta > 0.5 {
			nanosMul++
		}
		// nanosMultiplied += int64(float64(l.GetNanos()) * (float64(decMul) / float64(powerOf10)))
		nanosMultiplied += nanosMul
	}

	intUnitsMultiplied := l.GetUnits() * intMul
	decUnitsMultiplied := int64(0)
	if decMul != 0 {
		intUnitsMultiplied += int64(float64(l.GetUnits()*decMul) / float64(powerOf10))
		decUnitsMultiplied = l.GetUnits() * decMul % powerOf10
	}

	// For multiplication operations, if one operand is small (of the order 1e-08),
	//then int64 conversion would result in value 0.
	// For example, int64(math.Pow10(9)/float64(powerOf10)) would be 0.
	// To handle such scenarios and retain best value, perform multiplication across all operands involved,
	// followed by casting to int64.
	nanosDecUnitAdjusted := int64(float64(decUnitsMultiplied) * math.Pow10(9) / float64(powerOf10))

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

func MulNew(l *Money, r float64) (*Money, error) {
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
	powerOf10 := int64(math.Pow10(int(multiplierDecPlaces)))

	intMulF, decMulF := math.Modf(r)
	intMul, decMul := int64(intMulF), int64(decMulF*float64(powerOf10))
	// To handle edge scenarios where `decMulF*float64(powerOf10)` returns different value than expected.
	// For example: decMulF = 0.29 and powerOf10 = 100 should give 29 rather than 28.
	// Ensure the following invariant is true: decMulF == float64(decMul) / float64(powerOf10)
	// Increment decimal multipler (decMul) if deviation is >= 1%
	newDecMulF := float64(decMul) / float64(powerOf10)
	if newDecMulF < decMulF {
		percentageChange := ((decMulF - newDecMulF) / decMulF) * 100
		if percentageChange >= 1 {
			decMul++
		}
	}
	// multiply both sections
	nanosMultiplied := int64(l.GetNanos()) * intMul
	if decMul != 0 {
		// For expressions like: a * b / c, if a and b are of type int64,
		// then a * b could possibly exceed valid int64 range.
		// To handle such scenarios and retain best value,
		// perform division followed by multiplication: a * (b / c)
		nanosMultiplied += int64(float64(l.GetNanos()) * (float64(decMul) / float64(powerOf10)))
	}

	intUnitsMultiplied := l.GetUnits() * intMul
	decUnitsMultiplied := int64(0)
	if decMul != 0 {
		intUnitsMultiplied += int64(float64(l.GetUnits()*decMul) / float64(powerOf10))
		decUnitsMultiplied = l.GetUnits() * decMul % powerOf10
	}

	// For multiplication operations, if one operand is small (of the order 1e-08),
	//then int64 conversion would result in value 0.
	// For example, int64(math.Pow10(9)/float64(powerOf10)) would be 0.
	// To handle such scenarios and retain best value, perform multiplication across all operands involved,
	// followed by casting to int64.
	nanosDecUnitAdjusted := int64(float64(decUnitsMultiplied) * math.Pow10(9) / float64(powerOf10))

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

func Mul(l *Money, r float64) (*Money, error) {
	//fmt.Println("input:", l, r)
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
	//fmt.Println("power:", multiplierDecPlaces, powerOf10)

	intMulF, decMulF := math.Modf(r)
	intMul, decMul := int64(intMulF), int64(decMulF*float64(powerOf10))

	//fmt.Println("f:", intMulF, decMulF, intMul, decMul)

	// To handle edge scenarios where `decMulF*float64(powerOf10)` returns different value than expected.
	// For example: decMulF = 0.29 and powerOf10 = 100 should give 29 rather than 28.
	// Ensure the following invariant is true: decMulF == float64(decMul) / float64(powerOf10)
	// Increment decimal multipler (decMul) if deviation is >= 1%
	newDecMulF := float64(decMul) / float64(powerOf10)
	if newDecMulF < decMulF {
		percentageChange := ((decMulF - newDecMulF) / decMulF) * 100
		if percentageChange >= 1 {
			decMul++
		}
	}

	//fmt.Println("new:", newDecMulF, decMul)

	// multiply both sections
	nanosMultiplied := int64(l.GetNanos()) * intMul
	//fmt.Println("nanosmultiplied1:", nanosMultiplied)

	if decMul != 0 {
		nanosMultiplied += int64(l.GetNanos()) * decMul / int64(powerOf10)
		//fmt.Println("nanosmultiplied2:", nanosMultiplied, decMul, int64(powerOf10), decMul/int64(powerOf10), int64(l.GetNanos())*decMul/int64(powerOf10))

	}

	intUnitsMultiplied := l.GetUnits() * intMul
	decUnitsMultiplied := int64(0)
	if decMul != 0 {
		intUnitsMultiplied += int64(float64(l.GetUnits()*decMul) / float64(powerOf10))
		decUnitsMultiplied = l.GetUnits() * decMul % int64(powerOf10)
	}

	nanosDecUnitAdjusted := decUnitsMultiplied * int64(math.Pow10(9)/float64(powerOf10))

	//fmt.Println("adjusted:", nanosDecUnitAdjusted, decUnitsMultiplied)

	units := intUnitsMultiplied
	nanos := nanosDecUnitAdjusted + nanosMultiplied

	//fmt.Println("final:", units, nanos)

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
	//fmt.Println("final corrected:", units, nanos)

	return &Money{
		Units:        units,
		Nanos:        int32(nanos),
		CurrencyCode: l.GetCurrencyCode()}, nil
}

func generateMicro() {
	for i := 5; i < 2000; i++ {
		for j := 1100; j < 2000; j++ {
			fmt.Printf("%d,%d\n", i, j)
		}
	}
}

func generateSmall() {
	for i := 700; i < 2000; i++ {
		for j := 1510; j < 1600; j++ {
			fmt.Printf("%d,%d\n", i, j)
		}
	}
}

func generateBig() {
	for i := 19000; i < 20000; i++ {
		for j := 1500; j < 1600; j++ {
			fmt.Printf("%d,%d\n", i, j)
		}
	}
}

func test1() {
	l := &Money{Units: 19, Nanos: 130000000, CurrencyCode: ""}
	v := 15.11

	l2, err := Mul(l, DivideBy100(v))
	fmt.Println(err, l, l2)
}

func convertNanos(val string) int32 {
	var sb strings.Builder
	sb.Grow(10)
	sb.WriteString(val)
	for sb.Len() < 9 {
		sb.WriteRune('0')
	}
	i, _ := strconv.ParseInt(sb.String(), 10, 32)
	return int32(i)
}

func convertToMoney(val string) *Money {
	vals := strings.Split(val, ".")
	if len(vals) == 1 {
		vals = append(vals, "")
	}
	units, _ := strconv.ParseInt(vals[0], 10, 64)
	return &Money{
		Units:        units,
		Nanos:        convertNanos(vals[1]),
		CurrencyCode: "",
	}
}

func ReadCsvFile(filePath string, offset int) {
	// Load a csv file.
	f, _ := os.Open(filePath)

	// Create a new reader.
	r := csv.NewReader(f)
	for {
		record, err := r.Read()
		// Stop at EOF.
		if err == io.EOF {
			break
		}

		if err != nil {
			panic(err)
		}
		// Display record.
		// ... Display record length.
		// ... Display all individual elements of the slice.
		//fmt.Println(record)
		m := convertToMoney(record[0+offset])
		vat, _ := strconv.ParseFloat(record[1+offset], 64)
		expected := convertToMoney(record[2+offset])
		//fmt.Println("input:", m, vat)
		res, err := Mulv2(m, vat)
		if err != nil {
			fmt.Printf(err.Error())
			continue
		}
		if res.Units != expected.Units || res.Nanos != expected.Nanos {
			fmt.Println(m, vat, res, expected)
		} else {
			//fmt.Println("success: ", m, vat, res, expected)
		}
		//time.Sleep(1 * time.Second)
	}
}

func DivideBy100(v float64) float64 {
	//Note: Negative float not supported
	s := strconv.FormatFloat(v, 'G', -1, 64)
	vs := strings.Split(s, ".")
	beforeDecimal := (vs[0])
	afterDecimal := (vs[1])
	if len(beforeDecimal) == 0 {
		beforeDecimal = "00"
	} else if len(beforeDecimal) == 1 {
		beforeDecimal = "0" + beforeDecimal
	}

	var sb strings.Builder
	if len(beforeDecimal) > 2 {
		sb.WriteString(beforeDecimal[:len(beforeDecimal)-2])
	}
	sb.WriteRune('.')
	sb.WriteString(beforeDecimal[len(beforeDecimal)-2:]) //2 digit moved after decimal
	sb.WriteString(afterDecimal)
	res, _ := strconv.ParseFloat(sb.String(), 64)
	return res
}

func main() {
	//generateMicro()
	//generateSmall()
	//generateBig()
	//test1()
	ReadCsvFile("./small_test.csv", 0)
	ReadCsvFile("./big_test.csv", 0)
	ReadCsvFile("./big_test2.csv", 0)
	ReadCsvFile("./micro_test.csv", 2)
	//ReadCsvFile("./temp.csv")
	//fmt.Println(DivideBy100(15.11))
	//fmt.Println(DivideBy100(0.0012), DivideBy100(5433435.12))
}
