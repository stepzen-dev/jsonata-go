// Copyright 2018 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package jlib

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/stepzen-dev/jsonata-go/jlib/jxpath"
	"github.com/stepzen-dev/jsonata-go/jtypes"
)

// String converts a JSONata value to a string. Values that are
// already strings are returned unchanged. Functions return empty
// strings. All other types return their JSON representation.
func String(value interface{}) (string, error) {

	switch v := value.(type) {
	case jtypes.Callable:
		return "", nil
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case float64:
		// Will this ever fire in real world JSONata? Out of range
		// errors should be caught either at the parse stage or when
		// the argument to string() is evaluated. Tempted to remove
		// this test as Encode would catch the error anyway.
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", newError("string", ErrNaNInf)
		}
	}

	// TODO: Round numbers to 13dps to match jsonata-js.
	b := bytes.Buffer{}
	e := json.NewEncoder(&b)
	if err := e.Encode(value); err != nil {
		return "", err
	}

	// TrimSpace removes the newline appended by Encode.
	return strings.TrimSpace(b.String()), nil
}

// Substring returns the portion of a string starting at the
// given (zero-indexed) offset. Negative offsets count from the
// end of the string, e.g. a start position of -1 returns the
// last character. The optional third argument controls the
// maximum number of characters returned. By default, Substring
// returns all characters up to the end of the string.
func Substring(s string, start int, length jtypes.OptionalInt) string {

	if (length.IsSet() && length.Int <= 0) || start >= utf8.RuneCountInString(s) {
		return ""
	}

	if start < 0 {
		start += utf8.RuneCountInString(s)
	}

	if start > 0 {
		pos := positionOfNthRune(s, start)
		s = s[pos:]
	}

	if length.IsSet() && length.Int < utf8.RuneCountInString(s) {
		pos := positionOfNthRune(s, length.Int)
		s = s[:pos]
	}

	return s
}

// SubstringBefore returns the portion of a string that precedes
// the first occurrence of the given substring. If the substring
// is not present, SubstringBefore returns the full string.
func SubstringBefore(s, substr string) string {
	if i := strings.Index(s, substr); i >= 0 {
		return s[:i]
	}
	return s
}

// SubstringAfter returns the portion of a string that follows
// the first occurrence of the given substring. If the substring
// is not present, SubstringAfter returns the full string.
func SubstringAfter(s, substr string) string {
	if i := strings.Index(s, substr); i >= 0 {
		return s[i+len(substr):]
	}
	return s
}

// Pad returns a string padded to the specified number of characters.
// If the width is greater than zero, the string is padded to the
// right. If the width is less than zero, the string is padded to
// the left. The optional third argument specifies the characters
// used for padding. The default padding character is a space.
func Pad(s string, width int, chars jtypes.OptionalString) string {

	padlen := abs(width) - utf8.RuneCountInString(s)
	if padlen <= 0 {
		return s
	}

	ch := chars.String
	if ch == "" {
		ch = " "
	}

	padding := strings.Repeat(ch, padlen)
	if utf8.RuneCountInString(padding) > padlen {
		pos := positionOfNthRune(padding, padlen)
		padding = padding[:pos]
	}

	if width < 0 {
		return padding + s
	}

	return s + padding
}

var reWhitespace = regexp.MustCompile(`\s+`)

// Trim replaces consecutive whitespace characters in a string
// with a single space and trims spaces from the ends of the
// resulting string.
func Trim(s string) string {
	return strings.TrimSpace(reWhitespace.ReplaceAllString(s, " "))
}

// Contains returns true if the source string matches a given
// pattern. The pattern can be a string or a regular expression.
func Contains(s string, pattern StringCallable) (bool, error) {

	switch v := pattern.toInterface().(type) {
	case string:
		return strings.Contains(s, v), nil
	case jtypes.Callable:
		matches, err := extractMatches(v, s, -1)
		if err != nil {
			return false, err
		}
		return len(matches) > 0, nil
	default:
		return false, fmt.Errorf("function contains takes a string or a regex")
	}
}

// Split returns an array of substrings generated by splitting
// a string on the provided separator. The separator can be a
// string or a regular expression.
//
// If the separator is not present in the source string, Split
// returns a single-value array containing the source string.
// If the separator is an empty string, Split returns an array
// containing one element for each character in the source string.
//
// The optional third argument specifies the maximum number of
// substrings to return. By default, Split returns all substrings.
func Split(s string, separator StringCallable, limit jtypes.OptionalInt) ([]string, error) {

	if limit.Int < 0 {
		return nil, fmt.Errorf("third argument of the split function must evaluate to a positive number")
	}

	var parts []string

	switch sep := separator.toInterface().(type) {
	case string:
		parts = strings.Split(s, sep)
	case jtypes.Callable:
		matches, err := extractMatches(sep, s, -1)
		if err != nil {
			return nil, err
		}
		pos := 0
		for _, m := range matches {
			parts = append(parts, s[pos:m.indexes[0]])
			pos = m.indexes[1]
		}
		parts = append(parts, s[pos:])
	default:
		return nil, fmt.Errorf("function split takes a string or a regex")
	}

	if limit.IsSet() && limit.Int < len(parts) {
		parts = parts[:limit.Int]
	}

	return parts, nil
}

// Join concatenates an array of strings into a single string.
// The optional second parameter is a separator inserted between
// each pair of values.
func Join(values reflect.Value, separator jtypes.OptionalString) (string, error) {

	if !jtypes.IsArrayOf(values, jtypes.IsString) {
		if s, ok := jtypes.AsString(values); ok {
			return s, nil
		}
		return "", fmt.Errorf("function join takes an array of strings")
	}

	var vs []string
	values = jtypes.Resolve(values)

	for i := 0; i < values.Len(); i++ {
		s, _ := jtypes.AsString(values.Index(i))
		vs = append(vs, s)
	}

	return strings.Join(vs, separator.String), nil
}

// Match returns an array of objects describing matches of a
// regular expression in the source string. Each object in the
// array has the following fields:
//
//	match - the substring matched by the regex
//	index - the starting offset of this match
//	groups - any captured groups for this match
//
// The optional third argument specifies the maximum number
// of matches to return. By default, Match returns all matches.
func Match(s string, pattern jtypes.Callable, limit jtypes.OptionalInt) ([]map[string]interface{}, error) {

	if limit.Int < 0 {
		return nil, fmt.Errorf("third argument of function match must evaluate to a positive number")
	}

	max := -1
	if limit.IsSet() {
		max = limit.Int
	}

	matches, err := extractMatches(pattern, s, max)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, len(matches))

	for i, m := range matches {
		result[i] = map[string]interface{}{
			"match":  m.value,
			"index":  m.indexes[0],
			"groups": m.groups,
		}
	}

	return result, nil
}

// Replace returns a copy of the source string with zero or more
// instances of the given pattern replaced by the value provided.
// The pattern can be a string or a regular expression. The optional
// fourth argument specifies the maximum number of replacements
// to make. By default, all instances of pattern are replaced.
//
// If pattern is a string, the replacement must also be a string.
// If pattern is a regular expression, the replacement can be a
// string or a Callable.
//
// When replacing a regular expression with a string, the replacement
// string can refer to the matched value with $0 and any captured
// groups with $N, where N is the order of the submatch (e.g. $1
// is the first submatch).
//
// When replacing a regular expression with a Callable, the Callable
// must take a single argument and return a string. The argument is
// an object of the same form returned by Match.
func Replace(src string, pattern StringCallable, repl StringCallable, limit jtypes.OptionalInt) (string, error) {

	if limit.Int < 0 {
		return "", fmt.Errorf("fourth argument of function replace must evaluate to a positive number")
	}

	max := -1
	if limit.IsSet() {
		max = limit.Int
	}

	switch pattern := pattern.toInterface().(type) {
	case string:
		return replaceString(src, pattern, repl, max)
	case jtypes.Callable:
		return replaceMatchFunc(src, pattern, repl, max)
	default:
		return "", fmt.Errorf("second argument of function replace must be a string or a regex")
	}
}

func replaceString(src string, pattern string, repl StringCallable, limit int) (string, error) {

	if pattern == "" {
		return "", fmt.Errorf("second argument of function replace can't be an empty string")
	}

	s, ok := repl.toInterface().(string)
	if !ok {
		return "", fmt.Errorf("third argument of function replace must be a string when pattern is a string")
	}

	return strings.Replace(src, pattern, s, limit), nil
}

func replaceMatchFunc(src string, fn jtypes.Callable, repl StringCallable, limit int) (string, error) {

	var f jtypes.Callable
	var srepl string
	var expandable bool

	switch repl := repl.toInterface().(type) {
	case string:
		srepl = repl
		expandable = strings.ContainsRune(srepl, '$')
	case jtypes.Callable:
		f = repl
	default:
		return "", fmt.Errorf("third argument of function replace must be a string or a function")
	}

	matches, err := extractMatches(fn, src, limit)
	if err != nil {
		return "", err
	}

	for i := len(matches) - 1; i >= 0; i-- {

		var repl string

		if f != nil {
			repl, err = callReplaceFunc(f, matches[i])
			if err != nil {
				return "", err
			}
		} else {
			repl = srepl
			if expandable {
				repl = expandReplaceString(repl, matches[i])
			}
		}

		src = src[:matches[i].indexes[0]] + repl + src[matches[i].indexes[1]:]
	}

	return src, nil
}

var defaultDecimalFormat = jxpath.NewDecimalFormat()

// FormatNumber converts a number to a string, formatted according
// to the given picture string. See the XPath function format-number
// for the syntax of the picture string.
//
// https://www.w3.org/TR/xpath-functions-31/#formatting-numbers
//
// The optional third argument defines various formatting options
// such as the decimal separator and grouping separator. See the
// XPath documentation for details.
//
// https://www.w3.org/TR/xpath-functions-31/#defining-decimal-format
func FormatNumber(value float64, picture string, options jtypes.OptionalValue) (string, error) {

	if !options.IsSet() {
		return jxpath.FormatNumber(value, picture, defaultDecimalFormat)
	}

	opts := jtypes.Resolve(options.Value)
	if !jtypes.IsMap(opts) {
		return "", fmt.Errorf("decimal format options must be a map")
	}

	format, err := newDecimalFormat(opts)
	if err != nil {
		return "", err
	}

	return jxpath.FormatNumber(value, picture, format)
}

func newDecimalFormat(opts reflect.Value) (jxpath.DecimalFormat, error) {

	format := jxpath.NewDecimalFormat()

	for _, key := range opts.MapKeys() {

		k, ok := jtypes.AsString(key)
		if !ok {
			return jxpath.DecimalFormat{}, fmt.Errorf("decimal format options must be a map of strings to strings")
		}

		v, ok := jtypes.AsString(opts.MapIndex(key))
		if !ok {
			return jxpath.DecimalFormat{}, fmt.Errorf("decimal format options must be a map of strings to strings")
		}

		if err := updateDecimalFormat(&format, k, v); err != nil {
			return jxpath.DecimalFormat{}, err
		}
	}

	return format, nil
}

func updateDecimalFormat(format *jxpath.DecimalFormat, key string, value string) error {

	switch key {
	case "infinity":
		format.Infinity = value
	case "NaN":
		format.NaN = value
	case "percent":
		format.Percent = value
	case "per-mille":
		format.PerMille = value
	default:
		r, w := utf8.DecodeRuneInString(value)
		if r == utf8.RuneError || w != len(value) {
			return fmt.Errorf("invalid value %q for option %q", value, key)
		}
		switch key {
		case "decimal-separator":
			format.DecimalSeparator = r
		case "grouping-separator":
			format.GroupSeparator = r
		case "exponent-separator":
			format.ExponentSeparator = r
		case "minus-sign":
			format.MinusSign = r
		case "zero-digit":
			format.ZeroDigit = r
		case "digit":
			format.OptionalDigit = r
		case "pattern-separator":
			format.PatternSeparator = r
		default:
			return fmt.Errorf("unknown option %q", key)
		}
	}

	return nil
}

// FormatBase returns the string representation of a number in the
// optional base argument. If specified, the base must be between
// 2 and 36. By default, FormatBase uses base 10.
func FormatBase(value float64, base jtypes.OptionalFloat64) (string, error) {

	radix := 10
	if base.IsSet() {
		radix = int(Round(base.Float64, jtypes.OptionalInt{}))
	}

	if radix < 2 || radix > 36 {
		return "", fmt.Errorf("the second argument to formatBase must be between 2 and 36")
	}

	return strconv.FormatInt(int64(Round(value, jtypes.OptionalInt{})), radix), nil
}

// Base64Encode returns the base 64 encoding of a string.
func Base64Encode(s string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(s)), nil
}

// Base64Decode returns the string represented by a base 64 string.
func Base64Decode(s string) (string, error) {

	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// DecodeURL decodes a Uniform Resource Locator (URL)
// See https://docs.jsonata.org/string-functions#decodeurl
// and https://docs.jsonata.org/string-functions#decodeurlcomponent
func DecodeURL(s string) (string, error) {
	escaped, err := url.QueryUnescape(s)
	if err != nil {
		return "", err
	}
	return escaped, nil
}

// EncodeURL decodes a Uniform Resource Locator (URL)
// See https://docs.jsonata.org/string-functions#encodeurl
func EncodeURL(s string) (string, error) {
	// Go will encode the UTF-8 replacement character to %EF%BF%DB
	// but jsonata-js expects the operation to fail, so we'll
	// provide the same behavior
	if s == "�" {
		return "", fmt.Errorf("invalid character")
	}

	baseURL, err := url.Parse(s)
	if err != nil {
		return "", err
	}

	baseURL.RawQuery = baseURL.Query().Encode()
	return baseURL.String(), nil
}

// EncodeURLComponent decodes a component of a Uniform Resource Locator (URL)
// and https://docs.jsonata.org/string-functions#encodeurlcomponent
func EncodeURLComponent(s string) (string, error) {
	// Go will encode the UTF-8 replacement character to %EF%BF%DB
	// but jsonata-js expects the operation to fail, so we'll
	// provide the same behavior
	if s == "�" {
		return "", fmt.Errorf("invalid character")
	}

	return url.QueryEscape(s), nil
}

type match struct {
	value   string
	indexes [2]int
	groups  []string
}

func extractMatches(fn jtypes.Callable, s string, limit int) ([]match, error) {

	matches, err := callMatchFunc(fn, []reflect.Value{reflect.ValueOf(s)}, nil)
	if err != nil {
		return nil, err
	}

	if limit >= 0 && limit < len(matches) {
		matches = matches[:limit]
	}

	return matches, nil
}

func callMatchFunc(fn jtypes.Callable, argv []reflect.Value, matches []match) ([]match, error) {

	res, err := fn.Call(argv)
	if err != nil {
		return nil, err
	}

	if !res.IsValid() {
		return matches, nil
	}

	if !jtypes.IsMap(res) {
		return nil, fmt.Errorf("match function must return an object")
	}

	res = jtypes.Resolve(res)

	v := res.MapIndex(reflect.ValueOf("match"))
	value, ok := jtypes.AsString(v)
	if !ok {
		return nil, fmt.Errorf("match function must return an object with a string value named 'match'")
	}

	v = res.MapIndex(reflect.ValueOf("start"))
	start, ok := jtypes.AsNumber(v)
	if !ok {
		return nil, fmt.Errorf("match function must return an object with a number value named 'start'")
	}

	v = res.MapIndex(reflect.ValueOf("end"))
	end, ok := jtypes.AsNumber(v)
	if !ok {
		return nil, fmt.Errorf("match function must return an object with a number value named 'end'")
	}

	v = res.MapIndex(reflect.ValueOf("groups"))
	if !jtypes.IsArrayOf(v, jtypes.IsString) {
		return nil, fmt.Errorf("match function must return an object with a string array value named 'groups'")
	}

	v = jtypes.Resolve(v)
	groups := make([]string, v.Len())
	for i := range groups {
		s, _ := jtypes.AsString(v.Index(i))
		groups[i] = s
	}

	v = res.MapIndex(reflect.ValueOf("next"))
	next, ok := jtypes.AsCallable(v)
	if !ok {
		return nil, fmt.Errorf("match function must return an object with a Callable value named 'next'")
	}

	return callMatchFunc(next, nil, append(matches, match{
		value: value,
		indexes: [2]int{
			int(start),
			int(end),
		},
		groups: groups,
	}))
}

func expandReplaceString(s string, m match) string {

	var result string

Loop:
	for {
		pos := strings.IndexRune(s, '$')
		if pos == -1 {
			result += s
			break
		}

		result += s[:pos]
		s = s[pos+1:]

		if len(s) == 0 {
			result += "$"
			break
		}

		r, _ := utf8.DecodeRuneInString(s)

		if r == '$' || r < '0' || r > '9' {
			result += "$"
			if r == '$' {
				s = s[1:]
			}
			continue
		}

		if r == '0' {
			// $0 represents the full matched text.
			result += m.value
			s = s[1:]
			continue
		}

		var digits []rune

		for _, r := range s {
			if r < '0' || r > '9' {
				break
			}
			digits = append(digits, r)
		}

		indexes := runesToNumbers(digits)

		for i := len(indexes) - 1; i >= 0; i-- {
			// $N (where N > 0) represents the captured group with
			// index N-1.
			index := indexes[i] - 1
			if index < len(m.groups) {
				result += m.groups[index]
				s = s[i+1:]
				continue Loop
			}
		}

		s = s[1:]
	}

	return result
}

func callReplaceFunc(f jtypes.Callable, m match) (string, error) {

	data := map[string]interface{}{
		"match":  m.value,
		"index":  m.indexes[0],
		"groups": m.groups,
	}

	v, err := f.Call([]reflect.Value{reflect.ValueOf(data)})
	if err != nil {
		return "", err
	}

	repl, ok := jtypes.AsString(v)
	if !ok {
		return "", fmt.Errorf("third argument of function replace must be a function that returns a string")
	}

	return repl, nil
}

func runesToNumbers(runes []rune) []int {

	nums := make([]int, len(runes))

	for i := range runes {
		for j := 0; j <= i; j++ {
			nums[i] = nums[i]*10 + int(runes[j]-'0')
		}
	}

	return nums
}

func positionOfNthRune(s string, n int) int {

	i := 0
	for pos := range s {
		if i == n {
			return pos
		}
		i++
	}

	return -1
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
