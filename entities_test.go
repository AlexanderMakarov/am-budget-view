package main

import (
	"testing"
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