package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func Test_NewGroupExtractorByDetailsSubstrings(t *testing.T) {

	// Cases
	tests := []struct {
		name                          string
		groupNamesToSubstrings        map[string][]string
		isGroupAllUnknownTransactions bool
		expectedSubstringsToGroupName map[string]string
	}{
		{
			"no_groups_all_unknown",
			map[string][]string{},
			false,
			map[string]string{},
		},
		{
			"no_groups_1_unknown",
			map[string][]string{},
			true,
			map[string]string{},
		},
		{
			"many_groups_all_unknown",
			groups1,
			false,
			func() map[string]string {
				m := map[string]string{}
				m["a"] = "g1"
				m["b"] = "g2"
				m["c"] = "g2"
				m["d"] = "g3"
				return m
			}(),
		},
	}
	const testName = "NewGroupExtractorByDetailsSubstrings()"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Act
			builder, err := NewStatisticBuilderByDetailsSubstrings(tt.groupNamesToSubstrings,
				tt.isGroupAllUnknownTransactions, []string{})
			actualGE := builder(now, nowPlusMonth)

			// Assert
			if err != nil {
				t.Errorf("%s failed: %#v", testName, err)
			}
			if actualGE == nil {
				t.Errorf("%s builder returned null", testName)
			}
			groupNamesToSubstrings := actualGE.(GroupExtractorByCategories).groupNamesToSubstrings
			if !reflect.DeepEqual(groupNamesToSubstrings, tt.groupNamesToSubstrings) {
				t.Errorf("%s builder set wrong groupNamesToSubstrings: expected=%+v, actual=%+v", testName,
					tt.groupNamesToSubstrings, groupNamesToSubstrings)
			}
			substringsToGroupName := actualGE.(GroupExtractorByCategories).substringsToGroupName
			if !reflect.DeepEqual(substringsToGroupName, tt.expectedSubstringsToGroupName) {
				t.Errorf("%s builder set wrong substringsToGroupName: expected=%+v, actual=%+v", testName,
					tt.expectedSubstringsToGroupName, substringsToGroupName)
			}
		})
	}
}

func Test_groupExtractorByDetailsSubstrings_HandleTransaction(t *testing.T) {
	tI1a := newT(1, false, "a")
	tE1b := newT(1, true, "b")
	tI2c := newT(1, false, "c")
	tI3d := newT(1, false, "d")
	tI4c := newT(1, false, "c")
	tE2b := newT(1, true, "b")
	tE3b := newT(1, true, "b")
	tE4c := newT(1, true, "c")
	tI5b := newT(1, false, "b")
	tI6e := newT(2, false, "e") // Put here "2" to check something other than 1.
	tE6e := newT(2, true, "e")
	tI7f := newT(1, false, "f")
	tE7f := newT(1, true, "f")
	transactions1 := []Transaction{tI1a, tE1b, tI2c, tI3d, tI4c, tE2b, tE3b, tE4c, tI5b, tI6e, tE6e, tI7f, tE7f}
	// For groups1 which is:
	// "g1": {"a"},
	// "g2": {"b", "c"},
	// "g3": {"d"},
	// we should get:
	// Income: {g1 1 [tI1a], g2 3 [tI2c,tI4c,tI5b], g3 1 [tI3d], e 2 [tI6e], f 1 [tI7f]}
	// Expenses: {g1 0 [], g2 4 [tE1b,tE2b,tE3b,tE4c], g3 0 [], e 2 [tE6e], f 1 [tE7f]}

	// fields is aggregator of parameters for `groupExtractorByDetailsSubstrings`.
	type fields struct {
		intervalStats          *IntervalStatistic
		groupNamesToSubstrings map[string][]string
		substringsToGroupName  map[string]string
		isGroupAllUnknown      bool
	}

	// newFields is a factory for `fields`.
	newFields := func(groupNamesToSubstrings map[string][]string, isGroupAllUnknown bool) fields {
		substringsToGroupName := map[string]string{}
		for name, substrings := range groupNamesToSubstrings {
			for _, substring := range substrings {
				substringsToGroupName[substring] = name
			}
		}
		return fields{
			newIntervalStatistic(),
			groupNamesToSubstrings,
			substringsToGroupName,
			isGroupAllUnknown,
		}
	}

	tests := []struct {
		name         string
		fields       fields
		transactions []Transaction
		expected     *IntervalStatistic
	}{
		{
			name:         "no_groups-common_unknown",
			fields:       newFields(map[string][]string{}, true),
			transactions: transactions1,
			expected: func() *IntervalStatistic {
				r := newIntervalStatistic()
				r.Income = map[string]*Group{}
				r.Income[UnknownGroupName] = &Group{
					Name:         UnknownGroupName,
					Total:        MoneyWith2DecimalPlaces{8},
					Transactions: []Transaction{tI1a, tI2c, tI3d, tI4c, tI5b, tI6e, tI7f},
				}
				r.Expense = map[string]*Group{}
				r.Expense[UnknownGroupName] = &Group{
					Name:         UnknownGroupName,
					Total:        MoneyWith2DecimalPlaces{7},
					Transactions: []Transaction{tE1b, tE2b, tE3b, tE4c, tE6e, tE7f},
				}
				return r
			}(),
		},
		{
			name:         "no_groups-personal_unknowns",
			fields:       newFields(map[string][]string{}, false),
			transactions: transactions1,
			expected: func() *IntervalStatistic {
				r := newIntervalStatistic()
				r.Income = map[string]*Group{
					"+1a": groupFromITs("+1a", []Transaction{tI1a}),
					"+1c": groupFromITs("+1c", []Transaction{tI2c, tI4c}),
					"+1d": groupFromITs("+1d", []Transaction{tI3d}),
					"+1b": groupFromITs("+1b", []Transaction{tI5b}),
					"+2e": groupFromITs("+2e", []Transaction{tI6e}),
					"+1f": groupFromITs("+1f", []Transaction{tI7f}),
				}
				r.Expense = map[string]*Group{
					"-1b": groupFromITs("-1b", []Transaction{tE1b, tE2b, tE3b}),
					"-1c": groupFromITs("-1c", []Transaction{tE4c}),
					"-2e": groupFromITs("-2e", []Transaction{tE6e}),
					"-1f": groupFromITs("-1f", []Transaction{tE7f}),
				}
				return r
			}(),
		},
		{
			name:         "many_groups-common_unknown",
			fields:       newFields(groups1, true),
			transactions: transactions1,
			expected: func() *IntervalStatistic {
				r := newIntervalStatistic()
				r.Income = map[string]*Group{
					"g1":             groupFromITs("g1", []Transaction{tI1a}),
					"g2":             groupFromITs("g2", []Transaction{tI2c, tI4c, tI5b}),
					"g3":             groupFromITs("g3", []Transaction{tI3d}),
					UnknownGroupName: groupFromITs(UnknownGroupName, []Transaction{tI6e, tI7f}),
				}
				r.Expense = map[string]*Group{
					"g2":             groupFromITs("g2", []Transaction{tE1b, tE2b, tE3b, tE4c}),
					UnknownGroupName: groupFromITs(UnknownGroupName, []Transaction{tE6e, tE7f}),
				}
				return r
			}(),
		},
		{
			name:         "many_groups-personal_unknowns",
			fields:       newFields(groups1, false),
			transactions: transactions1,
			expected: func() *IntervalStatistic {
				r := newIntervalStatistic()
				r.Income = map[string]*Group{
					"g1":  groupFromITs("g1", []Transaction{tI1a}),
					"g2":  groupFromITs("g2", []Transaction{tI2c, tI4c, tI5b}),
					"g3":  groupFromITs("g3", []Transaction{tI3d}),
					"+2e": groupFromITs("+2e", []Transaction{tI6e}),
					"+1f": groupFromITs("+1f", []Transaction{tI7f}),
				}
				r.Expense = map[string]*Group{
					"g2":  groupFromITs("g2", []Transaction{tE1b, tE2b, tE3b, tE4c}),
					"-2e": groupFromITs("-2e", []Transaction{tE6e}),
					"-1f": groupFromITs("-1f", []Transaction{tE7f}),
				}
				return r
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Arrange
			handler := GroupExtractorByCategories{
				intervalStats:          tt.fields.intervalStats,
				groupNamesToSubstrings: tt.fields.groupNamesToSubstrings,
				substringsToGroupName:  tt.fields.substringsToGroupName,
				isGroupAllUnknown:      tt.fields.isGroupAllUnknown,
			}

			// Act
			for _, trans := range tt.transactions {
				if err := handler.HandleTransaction(trans); err != nil {
					t.Errorf("groupExtractorByDetailsSubstrings.HandleTransaction() failed on %v with %#v", trans, err)
				}
			}

			// Assert
			assertIntervalStatisticEqual(tt.expected, tt.fields.intervalStats, t)
		})
	}
}

func assertIntervalStatisticEqual(expected, actual *IntervalStatistic, t *testing.T) {
	if !expected.Start.Equal(actual.Start) {
		t.Errorf("Start time does not match. Expected: %v, got: %v", expected.Start, actual.Start)
		return
	}
	if !expected.End.Equal(actual.End) {
		t.Errorf("End time does not match. Expected: %v, got: %v", expected.End, actual.End)
		return
	}
	compareGroupMap("Income", expected.Income, actual.Income, t)
	compareGroupMap("Expense", expected.Expense, actual.Expense, t)
}

func compareGroupMap(name string, expected, actual map[string]*Group, t *testing.T) {
	if len(expected) != len(actual) {
		t.Errorf("%s maps have different lengths - %d instead of %d. Expected keys: %v, Actual keys: %v",
			name, len(actual), len(expected), getMapKeys(expected), getMapKeys(actual))
		return
	}
	for k, v := range expected {
		v2, ok := actual[k]
		if !ok {
			t.Errorf("%s map missing key: %s", name, k)
			continue
		}
		if !equalGroup(v, v2) {
			t.Errorf("%s group not equal for key %s. Expected:\n%+v\nActual:\n%+v", name, k, v, v2)
		}
	}
	for k, group := range actual {
		if _, ok := expected[k]; !ok {
			t.Errorf("%s map has extra key '%s' with group %+v", name, k, group)
		}
	}
}

func getMapKeys(m map[string]*Group) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func equalGroup(a, b *Group) bool {
	if a.Name != b.Name {
		return false
	}

	// Compare totals
	if a.Total.int != b.Total.int {
		return false
	}

	// Compare transactions slice lengths
	if len(a.Transactions) != len(b.Transactions) {
		return false
	}

	// Compare each transaction
	for i, transA := range a.Transactions {
		transB := b.Transactions[i]
		if !equalTransaction(transA, transB) {
			return false
		}
	}

	return true
}

func equalTransaction(a, b Transaction) bool {
	return a.Date == b.Date &&
		a.Details == b.Details &&
		a.Amount.int == b.Amount.int
}

var (
	now          = time.Now()
	nowPlusMonth = now.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)
	groups1      = map[string][]string{
		"g1": {"a"},
		"g2": {"b", "c"},
		"g3": {"d"},
	}
)

func newIntervalStatistic() *IntervalStatistic {
	return &IntervalStatistic{
		Start:   now,
		End:     nowPlusMonth,
		Income:  make(map[string]*Group),
		Expense: make(map[string]*Group),
	}
}

func newT(id int, isExpense bool, details string) Transaction {
	sign := "+"
	if isExpense {
		sign = "-"
	}
	return Transaction{
		IsExpense: isExpense,
		Date:      now,
		Details:   fmt.Sprintf("%s%d%s", sign, id, details),
		Amount:    MoneyWith2DecimalPlaces{id},
	}
}

func groupFromITs(name string, its []Transaction) *Group {
	sum := 0
	for _, trans := range its {
		sum += trans.Amount.int
	}
	return &Group{
		Name:         name,
		Total:        MoneyWith2DecimalPlaces{sum},
		Transactions: its,
	}
}
