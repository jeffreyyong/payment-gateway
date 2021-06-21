package luhn

import (
	"errors"
	"strconv"
)

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
