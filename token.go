package main

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

type idTokenFromDefaultTokenSource struct {
	TokenSource oauth2.TokenSource
}

func (s *idTokenFromDefaultTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.TokenSource.Token()
	if err != nil {
		return nil, err
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("missing id_token")
	}

	return &oauth2.Token{
		AccessToken: idToken,
		Expiry:      token.Expiry,
	}, nil
}

// Try to use the idtoken package, which will use the metadata service.
// However, the idtoken package does not work with gcloud's ADC, so we need to
// handle that case by falling back to default ADC search. However, the
// default ADC has a token at a different path, so we construct a custom token
// source for this edge case.
func findToken(ctx context.Context, audience string) (oauth2.TokenSource, error) {

	tokenSource, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		// If we got this far, it means that we found ADC, but the ADC was supplied
		// by a gcloud "authorized_user" instead of a service account. Thus we
		// fallback to the default ADC search.
		tokenSource, err = google.DefaultTokenSource(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get default token source: %w", err)
		}
		tokenSource = &idTokenFromDefaultTokenSource{TokenSource: tokenSource}
	}
	return oauth2.ReuseTokenSource(nil, tokenSource), nil
}
