package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

const googleOIDCDiscoveryURL = "https://accounts.google.com/.well-known/openid-configuration"

type googleDiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type googleAuthenticator struct {
	cfg     *Config
	oauth   *oauth2.Config
	issuer  string
	keyfunc keyfunc.Keyfunc
}

func newGoogleAuthenticator(ctx context.Context, cfg *Config) (*googleAuthenticator, error) {
	discovery, err := fetchDiscoveryDocument(ctx, http.DefaultClient)
	if err != nil {
		return nil, err
	}

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{discovery.JWKSURI})
	if err != nil {
		return nil, err
	}

	return &googleAuthenticator{
		cfg: cfg,
		oauth: &oauth2.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			RedirectURL:  cfg.Google.RedirectURL,
			Scopes:       []string{"openid", "email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  discovery.AuthorizationEndpoint,
				TokenURL: discovery.TokenEndpoint,
			},
		},
		issuer:  discovery.Issuer,
		keyfunc: kf,
	}, nil
}

func fetchDiscoveryDocument(ctx context.Context, client *http.Client) (*googleDiscoveryDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleOIDCDiscoveryURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned %s", resp.Status)
	}

	var doc googleDiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (a *googleAuthenticator) authURL(userID uint64, guildID uint64) (string, error) {
	state, err := signState(a.cfg.CookieSecret, tokenState{
		UserID:   userID,
		GuildID:  guildID,
		ExpireAt: time.Now().Add(time.Duration(a.cfg.TokenExpirySeconds) * time.Second).Unix(),
	})
	if err != nil {
		return "", err
	}

	return a.oauth.AuthCodeURL(
		state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("hd", "umich.edu"),
	), nil
}

type googleIDTokenClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	HostedDomain  string `json:"hd"`
	jwt.RegisteredClaims
}

func (a *googleAuthenticator) verifyCode(ctx context.Context, stateToken string, code string) (tokenState, error) {
	state, err := validateState(a.cfg.CookieSecret, stateToken, time.Now())
	if err != nil {
		return tokenState{}, err
	}

	token, err := a.oauth.Exchange(ctx, code)
	if err != nil {
		return tokenState{}, err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return tokenState{}, errors.New("missing id_token")
	}

	if err := a.validateIDToken(ctx, rawIDToken); err != nil {
		return tokenState{}, err
	}

	return state, nil
}

func (a *googleAuthenticator) validateIDToken(ctx context.Context, raw string) error {
	claims := googleIDTokenClaims{}
	_, err := jwt.ParseWithClaims(
		raw,
		&claims,
		a.keyfunc.KeyfuncCtx(ctx),
		jwt.WithIssuer(a.issuer),
		jwt.WithAudience(a.cfg.Google.ClientID),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return err
	}

	if !claims.EmailVerified {
		return errors.New("email not verified")
	}

	address, err := mail.ParseAddress(claims.Email)
	if err != nil {
		return err
	}

	_, domain, ok := strings.Cut(strings.ToLower(address.Address), "@")
	if !ok || domain != "umich.edu" {
		return errors.New("email domain is not umich.edu")
	}

	if claims.HostedDomain != "umich.edu" {
		return errors.New("hosted domain is not umich.edu")
	}

	return nil
}
