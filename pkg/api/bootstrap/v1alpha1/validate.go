package v1alpha1

import (
	"errors"
	"fmt"
)

func (c *CreateTokenRequest) Validate() error {
	if c.TTL.AsDuration() == 0 {
		return errors.New("invalid duration")
	}
	return nil
}

func (d *DeleteTokenRequest) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("invalid token ID")
	}
	return nil
}
