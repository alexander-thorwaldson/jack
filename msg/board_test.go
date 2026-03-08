//go:build testing

package msg

import (
	"fmt"
)

func stubResolver(roomID string) AliasResolver {
	return func(_ string) (*AliasResponse, error) {
		return &AliasResponse{RoomID: roomID}, nil
	}
}

func failResolver() AliasResolver {
	return func(_ string) (*AliasResponse, error) {
		return nil, fmt.Errorf("not found")
	}
}

func stubCreator(roomID string) func(string, string, string) (*Room, error) {
	return func(_, _, _ string) (*Room, error) {
		return &Room{RoomID: roomID}, nil
	}
}
