package luhn

import (
	"errors"
	"strconv"
)

// Validate checks if a pan is all numeric first.
// It then performs the validation algorithm:
// 1. reverse the digits:
// 2. if it's an even number:
// 		a. times the digit by 2.
// 		b. sum the digits if it's greater than 9,
//	    	e.g. 16 will become 7 as it's 1 + 6
// 3. sum all the numbers together
// 4. if the sum ends in zero, it passes the validation else returns error.
func Validate(pan string) error {
	panNum, err := strconv.Atoi(pan)
	if err != nil {
		return errors.New("pan contains non numeric or spaces")
	}

	var luhn int
	for i := 1; panNum > 0; i++ {
		// get the last digit
		cur := panNum % 10

		if i%2 == 0 {
			cur = cur * 2
			if cur > 9 {
				// e.g. if number = 16,
				// 7 = 6 + 1
				cur = cur%10 + cur/10
			}
		}
		luhn += cur
		panNum = panNum / 10
	}

	if luhn%10 != 0 {
		return errors.New("luhn validation failed")
	}

	return nil
}
