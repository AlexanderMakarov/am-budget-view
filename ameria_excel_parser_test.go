package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestMoneyWith2DecimalPlaces_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantInt int
		wantErr bool
	}{
		{
			name:    "valid input",
			input:   "123.45",
			wantInt: 12345,
			wantErr: false,
		},
		{
			name:    "input with decimal places",
			input:   "123.456",
			wantInt: 12345,
			wantErr: false,
		},
		{
			name:    "input with negative value",
			input:   "-123.45",
			wantInt: -12345,
			wantErr: false,
		},
		{
			name:    "invalid input",
			input:   "abc",
			wantInt: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m MoneyWith2DecimalPlaces
			err := m.UnmarshalText([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error: got %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && m.int != tt.wantInt {
				t.Errorf("got int %d, want %d", m.int, tt.wantInt)
			}
		})
	}
}

func TestParseRawTransactionsFromFile(t *testing.T) {
	tests := []struct {
		name           string
		filePath       string
		myAccounts     []string
		detailsIncome  []string
		wantErr        bool
		expectedResult []Transaction
	}{
		{
			name:          "valid_file-check_by_account",
			filePath:      filepath.Join("testdata", "ameria", "valid_file.xls"),
			myAccounts:    []string{"1234567890123456"},
			detailsIncome: []string{},
			wantErr:       false,
			expectedResult: []Transaction{
				{
					IsExpense: true,
					Date:      time.Date(2024, time.April, 20, 0, 0, 0, 0, time.UTC),
					Details:   "ԱԱՀ այդ թվում` 16.67%",
					Amount:    MoneyWith2DecimalPlaces{int: 10010},
				},
				{
					IsExpense: false,
					Date:      time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC),
					Details:   "Բանկի ձևանմուշից տարբերվող տեղեկա",
					Amount:    MoneyWith2DecimalPlaces{int: 99999999999},
				},
			},
		},
		{
			name:           "file_not_found",
			filePath:       filepath.Join("testdata", "ameria", "non_existent_file.xls"),
			myAccounts:     []string{},
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			name:           "invalid_header",
			filePath:       filepath.Join("testdata", "ameria", "invalid_header.xls"),
			myAccounts:     []string{},
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			name:           "no_data",
			filePath:       filepath.Join("testdata", "ameria", "no_data.xls"),
			myAccounts:     []string{},
			detailsIncome:  []string{},
			wantErr:        false,
			expectedResult: []Transaction{},
		},
		{
			name:          "valid_file-check_by_details",
			filePath:      filepath.Join("testdata", "ameria", "valid_file.xls"),
			myAccounts:    []string{},
			detailsIncome: []string{"income"},
			wantErr:       false,
			expectedResult: []Transaction{
				{
					IsExpense: true,
					Date:      time.Date(2024, time.April, 20, 0, 0, 0, 0, time.UTC),
					Details:   "ԱԱՀ այդ թվում` 16.67%",
					Amount:    MoneyWith2DecimalPlaces{int: 10010},
				},
				{
					IsExpense: true, // I.e. recognition didn't work.
					Date:      time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC),
					Details:   "Բանկի ձևանմուշից տարբերվող տեղեկա",
					Amount:    MoneyWith2DecimalPlaces{int: 99999999999},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := MyAmeriaExcelFileParser{
				MyAccounts:              tt.myAccounts,
				DetailsIncomeSubstrings: tt.detailsIncome,
			}
			actual, err := parser.ParseRawTransactionsFromFile(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRawTransactionsFromFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(actual, tt.expectedResult) {
				t.Errorf("ParseRawTransactionsFromFile() = %v, want %v", actual, tt.expectedResult)
			}
		})
	}
}
