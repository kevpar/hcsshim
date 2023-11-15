package state

import (
	"encoding/json"
	"os"
)

func Write[T any](path string, state *T) error {
	j, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, j, 0644); err != nil {
		return err
	}
	return nil
}

func Read[T any](path string) (*T, error) {
	v := new(T)
	j, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(j, v); err != nil {
		return nil, err
	}
	return v, nil
}
