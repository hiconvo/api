package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hiconvo/api/utils/secrets"
)

var googleAud string = secrets.Get("GOOGLE_OAUTH_KEY", "")

type UserPayload struct {
	Email    string `validate:"regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]{2\\,4}$"`
	Provider string `validate:"regexp=^(google|facebook)$"`
	Token    string `validate:"nonzero"`
}

type ProviderPayload struct {
	ID        string
	Email     string
	FirstName string
	LastName  string
	Provider  string
}

// Verify both verifies the fiven oauth token and retrieves needed info about
// the user.
func Verify(ctx context.Context, payload UserPayload) (ProviderPayload, error) {
	// FIXME: If a user changes her email via an oauth party, we will not know
	// about it. Need to monitor for bounced emails and show message on web ui
	// to prompt user to update email in that case.
	if payload.Provider == "google" {
		return verifyGoogleToken(ctx, payload)
	}
	return verifyFacebookToken(ctx, payload)
}

func verifyGoogleToken(ctx context.Context, payload UserPayload) (ProviderPayload, error) {
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", payload.Token)
	res, err := http.Get(url)
	if err != nil {
		return ProviderPayload{}, err
	}

	data := make(map[string]string)
	if decodeErr := json.NewDecoder(res.Body).Decode(&data); decodeErr != nil {
		return ProviderPayload{}, decodeErr
	}

	if data["aud"] != googleAud {
		return ProviderPayload{}, errors.New("Aud did not match")
	}

	return ProviderPayload{
		ID:        data["sub"],
		Provider:  "google",
		Email:     data["email"],
		FirstName: data["given_name"],
		LastName:  data["family_name"],
	}, nil
}

func verifyFacebookToken(ctx context.Context, payload UserPayload) (ProviderPayload, error) {
	url := fmt.Sprintf(
		"https://graph.facebook.com/me?fields=id,email,first_name,last_name&access_token=%s",
		payload.Token)
	res, err := http.Get(url)
	if err != nil {
		return ProviderPayload{}, err
	}

	data := make(map[string]string)
	if decodeErr := json.NewDecoder(res.Body).Decode(&data); decodeErr != nil {
		return ProviderPayload{}, decodeErr
	}

	return ProviderPayload{
		ID:        data["id"],
		Provider:  "facebook",
		Email:     data["email"],
		FirstName: data["first_name"],
		LastName:  data["last_name"],
	}, nil
}
