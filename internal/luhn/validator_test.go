package luhn_test

import (
	"testing"

	"github.com/jeffreyyong/payment-gateway/internal/luhn"
	"github.com/stretchr/testify/assert"
)

func TestValidator_Validate(t *testing.T) {
	testCases := []struct {
		description    string
		pan            string
		expectedErr    bool
		expectedErrMsg string
	}{
		{
			"valid",
			"49927398716",
			false,
			"",
		},
		{
			"luhn validation failed",
			"49927398717",
			true,
			"luhn validation failed",
		},
		{
			"contains alphabet",
			"ab1ldaf716",
			true,
			"pan contains non numeric or spaces",
		},
		{
			"valid",
			"1234567812345670",
			false,
			"",
		},
		{
			"luhn validation failed",
			"1234567812345678",
			true,
			"luhn validation failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := luhn.Validate(tc.pan)
			if tc.expectedErr {
				assert.EqualError(t, err, tc.expectedErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
