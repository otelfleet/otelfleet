package v1alpha1

import "errors"

func (c *CreateTokenRequest) Validate() error {
	if c.TTL.AsDuration() == 0 {
		return errors.New("invalid duration")
	}
	return nil
}
