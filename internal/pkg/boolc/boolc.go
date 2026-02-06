// Package boolc provides a set of variadic functions for boolean algebra operations.
// These utilities allow for clearer expressions when evaluating multiple conditions,
// avoiding deeply nested logical operators.
package boolc

// And returns true if ALL arguments are true.
// Returns true if no arguments are provided (identity element for conjunction).
func And(args ...bool) bool {
	for _, b := range args {
		if !b {
			return false
		}
	}
	return true
}

// Or returns true if AT LEAST ONE argument is true.
// Returns false if no arguments are provided (identity element for disjunction).
func Or(args ...bool) bool {
	for _, b := range args {
		if b {
			return true
		}
	}
	return false
}

// Not returns the logical negation of the argument.
func Not(a bool) bool {
	return !a
}

// Nand (Not AND) returns true if NOT ALL arguments are true.
// It is equivalent to !And(...).
func Nand(args ...bool) bool {
	return !And(args...)
}

// Nor (Not OR) returns true if NONE of the arguments are true.
// It is equivalent to !Or(...).
func Nor(args ...bool) bool {
	return !Or(args...)
}

// Xor (Exclusive OR) returns true if an ODD number of arguments are true.
// For two arguments, this effectively checks if they are different.
func Xor(args ...bool) bool {
	var count int
	for _, b := range args {
		if b {
			count++
		}
	}
	return count%2 != 0
}

// Xnor (Exclusive NOR) returns true if an EVEN number of arguments are true.
// For two arguments, this effectively checks if they are equal (equivalence).
func Xnor(args ...bool) bool {
	return !Xor(args...)
}
