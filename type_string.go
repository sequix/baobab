// Code generated by "stringer -type Type"; DO NOT EDIT.

package main

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[EOF-0]
	_ = x[Error-1]
	_ = x[LeftParen-2]
	_ = x[RightParen-3]
	_ = x[String-4]
	_ = x[Word-5]
}

const _Type_name = "EOFErrorLeftParenRightParenStringWord"

var _Type_index = [...]uint8{0, 3, 8, 17, 27, 33, 37}

func (i Type) String() string {
	if i < 0 || i >= Type(len(_Type_index)-1) {
		return "Type(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Type_name[_Type_index[i]:_Type_index[i+1]]
}
