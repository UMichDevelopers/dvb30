package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"
)

type tokenState struct {
	UserID   uint64
	GuildID  uint64
	ExpireAt int64
}

func signState(cookieSecret string, state tokenState) (string, error) {
	payload := stateMessage(state)
	sum := sha256.Sum256([]byte(cookieSecret))
	mac := hmac.New(sha256.New, sum[:])
	if _, err := mac.Write(payload); err != nil {
		return "", err
	}

	token := append(payload, mac.Sum(nil)...)
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func validateState(cookieSecret string, token string, now time.Time) (tokenState, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return tokenState{}, err
	}

	if len(raw) != 24+sha256.Size {
		return tokenState{}, errors.New("invalid state length")
	}

	payload := raw[:24]
	signature := raw[24:]

	state := tokenState{
		UserID:   binary.BigEndian.Uint64(payload[0:8]),
		GuildID:  binary.BigEndian.Uint64(payload[8:16]),
		ExpireAt: int64(binary.BigEndian.Uint64(payload[16:24])),
	}

	sum := sha256.Sum256([]byte(cookieSecret))
	mac := hmac.New(sha256.New, sum[:])
	if _, err := mac.Write(payload); err != nil {
		return tokenState{}, err
	}

	if !hmac.Equal(signature, mac.Sum(nil)) {
		return tokenState{}, errors.New("invalid state signature")
	}

	if now.Unix() > state.ExpireAt {
		return tokenState{}, errors.New("state expired")
	}

	return state, nil
}

func stateMessage(state tokenState) []byte {
	msg := make([]byte, 24)
	binary.BigEndian.PutUint64(msg[0:8], state.UserID)
	binary.BigEndian.PutUint64(msg[8:16], state.GuildID)
	binary.BigEndian.PutUint64(msg[16:24], uint64(state.ExpireAt))
	return msg
}
