package database

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	rTags, _ = regexp.Compile("^[a-z\\-0-9]+(,[a-z\\-0-9]+)*$")
)

type Tags []string

func (o *Tags) Scan(src any) error {
	tagString, ok := src.(string)
	if !ok {
		return errors.New("src value cannot cast to []string")
	}
	*o = strings.Split(tagString, ",")
	return nil
}

func (o Tags) Value() (driver.Value, error) {
	if len(o) == 0 {
		return nil, nil
	}

	if err := o.IsValid(); err != nil {
		return nil, err
	}

	return strings.ToLower(strings.Join(o, ",")), nil
}

func (tags *Tags) IsValid() error {
	if tags == nil {
		return nil
	}

	for _, tag := range *tags {
		if !rTags.MatchString(tag) {
			return fmt.Errorf("invalid tag: %s", tag)
		}
	}

	return nil
}
