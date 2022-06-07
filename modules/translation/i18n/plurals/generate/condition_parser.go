// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// This file is heavily inspired by https://github.com/nicksnyder/go-i18n/tree/main/v2/internal/plural

package generate

import (
	"fmt"
	"regexp"
	"strings"
)

// As noted above relation is a lot simpler than the original full rules imply:
//
// relation        = expr ('=' | '!=') range_list
// expr            = operand ('%' value)?
// operand         = 'n' | 'i' | 'f' | 't' | 'v' | 'w' | 'e'
//
var relationRegexp = regexp.MustCompile(`([nieftvw])(?:\s*%\s*([0-9]+))?\s*(!=|=)(.*)`)

// ConditionToGoString converts a CLDR plural rules to Go code.
// See http://unicode.org/reports/tr35/tr35-numbers.html#Plural_rules_syntax
func ConditionToGoString(condition string) string {
	// the BNF does not allow for ors to be inside ands
	// so a simple recursion will work

	// condition       = and_condition ('or' and_condition)*
	var parsedOrConditions []string
	for _, andCondition := range strings.Split(condition, "or") {
		andCondition = strings.TrimSpace(andCondition)
		if andCondition == "" {
			continue
		}
		var parsedAndConditions []string

		// again the BNF does not allow for ands to be inside relations
		// so a simple recursion will work

		// and_condition   = relation ('and' relation)*
		for _, relation := range strings.Split(andCondition, "and") {

			// Now although the full BNF allows for a more a complex set of relations
			// the files we are interested in are much simpler and their restricted BNF is
			// as below

			// relation        = expr ('=' | '!=') range_list
			// expr            = operand ('%' modValue)?
			// operand         = 'n' | 'i' | 'e' | 'f' | 't' | 'v' | 'w'

			// An operand here relates to how the input number N is after exponentiation is applied
			//
			// n	the absolute value of N.
			// i	the integer digits of N.
			// e  the exponent value of N.
			// v	the number of visible fraction digits in N, with trailing zeros.
			// w	the number of visible fraction digits in N, without trailing zeros.
			// f	the visible fraction digits in N, with trailing zeros, expressed as an integer.
			// t	the visible fraction digits in N, without trailing zeros, expressed as an integer.
			//
			// This implies that at least in some languages 1.3 and 1.30 could have different plural forms.
			parts := relationRegexp.FindStringSubmatch(relation)
			if parts == nil {
				continue
			}

			operand, modValue, relationType, ranges := strings.ToUpper(parts[1]), parts[2], parts[3], strings.TrimSpace(parts[4])

			// Now we want to convert the condition string to something which will evaluate

			// Now convert the operand to a field in the structure
			operand = "ops." + operand

			// ranges          = (range | value) (',' range_list)*
			// range           = from'..'to (value..value)
			// value           = digit+
			// digit           = [0-9]
			var parsedExprRanges []string
			var values []string
			for _, rangeValue := range strings.Split(ranges, ",") {
				// check if contains ..
				// range           = value'..'value
				if parts := strings.Split(rangeValue, ".."); len(parts) == 2 {
					from, to := parts[0], parts[1]

					// Now if we are testing the N operand because it could be a decimal number we need to use a different function
					if operand == "ops.N" {
						if modValue != "" {
							parsedExprRanges = append(parsedExprRanges, fmt.Sprintf("ops.NModInRange(%s, %s, %s)", modValue, from, to))
							continue
						}
						parsedExprRanges = append(parsedExprRanges, fmt.Sprintf("ops.NInRange(%s, %s)", from, to))
						continue
					}

					// Otherwise we can simply mod the operand value directly
					if modValue != "" {
						parsedExprRanges = append(parsedExprRanges, fmt.Sprintf("intInRange(%s %% %s, %s, %s)", operand, modValue, from, to))
					} else {
						parsedExprRanges = append(parsedExprRanges, fmt.Sprintf("intInRange(%s, %s, %s)", operand, from, to))
					}
					continue
				}

				// We have a plain value - collect them and test them together
				values = append(values, rangeValue)
			}

			if len(values) > 0 {
				valuesArgs := strings.Join(values, ",")

				// Now if we are testing the N operand because it could be a decimal number we need to use a different function
				if operand == "ops.N" {
					if modValue != "" {
						parsedExprRanges = append(parsedExprRanges, fmt.Sprintf("ops.NModEqualsAny(%s, %s)", modValue, valuesArgs))
					} else {
						parsedExprRanges = append(parsedExprRanges, fmt.Sprintf("ops.NEqualsAny(%s)", valuesArgs))
					}
				} else if modValue != "" {
					parsedExprRanges = append(parsedExprRanges, fmt.Sprintf("intEqualsAny(%s %% %s, %s)", operand, modValue, valuesArgs))
				} else {
					parsedExprRanges = append(parsedExprRanges, fmt.Sprintf("intEqualsAny(%s, %s)", operand, valuesArgs))
				}
			}

			// join all the parsed Ranges together as Ors
			parsedRelations := strings.Join(parsedExprRanges, " || ")

			// Group them
			if len(parsedExprRanges) > 1 {
				parsedRelations = "(" + parsedRelations + ")"
			}

			// Handle not
			if relationType == "!=" {
				parsedRelations = "!" + parsedRelations
			}

			parsedAndConditions = append(parsedAndConditions, parsedRelations)
		}
		parsedAndCondition := strings.TrimSpace(strings.Join(parsedAndConditions, " && "))
		if parsedAndCondition == "" {
			continue
		}
		parsedOrConditions = append(parsedOrConditions, parsedAndCondition)
	}
	return strings.Join(parsedOrConditions, " ||\n")
}
