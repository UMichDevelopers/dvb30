package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type tokenState struct {
	UserID   uint64
	GuildID  uint64
	ExpireAt int64
}

func signState(cookieSecret string, state tokenState) (string, error) {
	sum := sha256.Sum256([]byte(cookieSecret))
	mac := hmac.New(sha256.New, sum[:])
	if _, err := mac.Write(stateMessage(state)); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%d.%d.%d.%s",
		state.UserID,
		state.GuildID,
		state.ExpireAt,
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil)),
	), nil
}

func validateState(cookieSecret string, token string, now time.Time) (tokenState, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return tokenState{}, errors.New("invalid state format")
	}

	userID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return tokenState{}, err
	}

	guildID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return tokenState{}, err
	}

	expireAt, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return tokenState{}, err
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return tokenState{}, err
	}

	state := tokenState{
		UserID:   userID,
		GuildID:  guildID,
		ExpireAt: expireAt,
	}

	sum := sha256.Sum256([]byte(cookieSecret))
	mac := hmac.New(sha256.New, sum[:])
	if _, err := mac.Write(stateMessage(state)); err != nil {
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
