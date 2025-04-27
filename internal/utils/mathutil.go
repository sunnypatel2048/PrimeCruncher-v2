package utils

import "math/big"

// isPrime returns true if n is a prime number, and false otherwise
func IsPrime(n uint64) bool {
	bigInt := new(big.Int).SetUint64(n)
	return bigInt.ProbablyPrime(5)
}
