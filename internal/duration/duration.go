package duration

import (
	"encoding/json"
	"errors"
	"time"
)

type D struct {
	time.Duration
}

func New(d time.Duration) D {
	return D{Duration: d}
}

func (this D) MarshalJSON() ([]byte, error) {
	return json.Marshal(this.String())
}

func (this *D) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		this.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		this.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("unexpected type for duration")
	}
}
