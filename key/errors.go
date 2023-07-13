package key

import (
	"fmt"
)

func ErrInvalidKey(l int) error {
	return fmt.Errorf("invalid key: should be %d bytes long", l)
}
