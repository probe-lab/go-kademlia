package key

import (
	"errors"
	"fmt"
)

var ErrKeyWrongLength = errors.New("key has wrong length")

func ErrInvalidKey(l int) error {
	return fmt.Errorf("invalid key: should be %d bytes long", l)
}
