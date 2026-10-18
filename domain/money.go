package domain

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrInvalidMoneyFormat = errors.New("formato monetário inválido: deve ser um decimal válido com até 2 casas decimais")
	ErrCurrencyMismatch   = errors.New("incompatibilidade de moedas")
	ErrNegativeAmount 	  = errors.New("não é permitido valor negativo para operações")
	ErrIntegerOverflow 	  = errors.New("valor monetário excede o limite")
)

var decimalRegex = regexp.MustCompile(`^-?([0-9]+)(\.[0-9]{1,2})?$`)

type Money struct {
	amount int64		// Unidade mínimas (centavos)
	currency string		// Código ISO 4127
}

func NewMoney(amountStr, currency string) (Money, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return Money{}, errors.New("'currency' não pode ser vázio")
	}

	amountStr = strings.TrimSpace(amountStr)
	if !decimalRegex.MatchString(amountStr) {
		return Money{}, ErrInvalidMoneyFormat
	}

	cents, err := parseToCents(amountStr)
	if err != nil {
		return Money{}, err
	}

	return Money{amount: cents, currency: currency}, nil
}

func ZeroMoney(currency string) Money {
	return Money{amount: 0, currency: strings.ToUpper(strings.TrimSpace(currency))}
}

func (m Money) Amount() int64 {
	return m.amount
}
func (m Money) Currency() string {
	return m.currency
}
func (m Money) IsZero() bool {
	return m.amount == 0
}
func (m Money) IsPositive() bool {
	return m.amount > 0
}

func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}
	if (other.amount > 0 && m.amount > math.MaxInt64 - other.amount) || (other.amount < 0 && m.amount < math.MinInt64 - other.amount) {
		return Money{}, ErrIntegerOverflow
	}

	return Money{amount: m.amount + other.amount, currency: m.currency}, nil 
}

func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}
	if (other.amount < 0 && m.amount > math.MaxInt64 + other.amount) || (other.amount > 0 && m.amount < math.MinInt64 + other.amount) {
		return Money{}, ErrIntegerOverflow
	}

	return Money{amount: m.amount - other.amount, currency: m.currency}, nil
}

func (m Money) Negate() (Money, error) {
	if m.amount == math.MinInt64 {
		return Money{}, ErrIntegerOverflow
	}

	return Money{amount: -m.amount, currency: m.currency}, nil
}

func (m Money) Compare(other Money) (int, error) {
	if m.currency != other.currency {
		return 0, ErrCurrencyMismatch
	}
	if m.amount < other.amount {
		return -1, nil
	} else if m.amount > other.amount {
		return 1, nil
	}

	return 0, nil
}

func (m Money) String() string {
	abs := m.amount
	sign := ""
	
	if abs < 0 {
		sign = "-"
		abs = -abs
	}

	dollars := abs / 100
	cents := abs % 100
	return fmt.Sprintf("%s%d.%02d", sign, dollars, cents)
}

func (m Money) MarshalJSON() ([]byte, error) {
	str := m.String()
	return fmt.Appendf(nil, `{"amount":"%s", "currency":"%s"}`, str, m.currency), nil
}

func parseToCents(s string) (int64, error) {
	parts := strings.Split(s, ".")
	integerPartStr := parts[0]
	fractionalPartStr := "00"

	if len(parts) == 2 {
		fractionalPartStr = parts[1]
		if len(fractionalPartStr) == 1 {
			fractionalPartStr += "0"
		}
	}

	integerVal, err := strconv.ParseInt(integerPartStr, 10, 64)
	if err != nil {
		return 0, ErrInvalidMoneyFormat
	}

	fractionalVal, err := strconv.ParseInt(fractionalPartStr, 10, 64)
	if err != nil {
		return 0, ErrInvalidMoneyFormat
	}

	isNegative := strings.HasPrefix(s, "-")
	var totalCents int64
	if isNegative {
		totalCents = (integerVal * 100) - fractionalVal
	} else {
		totalCents = (integerVal * 100) + fractionalVal
	}

	return totalCents, nil
}