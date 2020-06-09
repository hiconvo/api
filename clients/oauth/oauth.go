package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hiconvo/api/errors"
)

type UserPayload struct {
	Provider string `validate:"regexp=^(google|facebook)$"`
	Token    string `validate:"nonzero"`
}

type ProviderPayload struct {
	ID         string
	Email      string
	FirstName  string
	LastName   string
	Provider   string
	TempAvatar string
}

type Client interface {
	Verify(context.Context, UserPayload) (ProviderPayload, error)
}

type clientImpl struct {
	googleAud string
}

func NewClient(googleAud string) Client {
	return &clientImpl{googleAud}
}

func (c *clientImpl) Verify(ctx context.Context, payload UserPayload) (ProviderPayload, error) {
	if payload.Provider == "google" {
		return c.verifyGoogleToken(ctx, payload)
	}

	return c.verifyFacebookToken(ctx, payload)
}

func (c *clientImpl) verifyGoogleToken(ctx context.Context, payload UserPayload) (ProviderPayload, error) {
	var op errors.Op = "oauth.verifyGoogleToken"

	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", payload.Token)

	res, err := http.Get(url)
	if err != nil {
		return ProviderPayload{}, errors.E(op, err)
	}

	data := make(map[string]string)
	if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
		return ProviderPayload{}, errors.E(op, err)
	}

	if data["aud"] != c.googleAud {
		return ProviderPayload{}, errors.E(op, http.StatusBadRequest, errors.Str("Aud did not match"))
	}

	return ProviderPayload{
		ID:         data["sub"],
		Provider:   "google",
		Email:      data["email"],
		FirstName:  data["given_name"],
		LastName:   data["family_name"],
		TempAvatar: data["picture"] + "?sz=256",
	}, nil
}

func (c *clientImpl) verifyFacebookToken(ctx context.Context, payload UserPayload) (ProviderPayload, error) {
	var op errors.Op = "oauth.verifyFacebookToken"

	url := fmt.Sprintf(
		"https://graph.facebook.com/me?fields=id,email,first_name,last_name&access_token=%s",
		payload.Token)

	res, err := http.Get(url)
	if err != nil {
		return ProviderPayload{}, errors.E(op, err)
	}

	data := make(map[string]interface{})
	if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
		return ProviderPayload{}, errors.E(op, err)
	}

	tempAvatarURI := fmt.Sprintf(
		"https://graph.facebook.com/%s/picture?type=large&width=256&height=256&access_token=%s",
		data["id"].(string), payload.Token)

	return ProviderPayload{
		ID:         data["id"].(string),
		Provider:   "facebook",
		Email:      data["email"].(string),
		FirstName:  data["first_name"].(string),
		LastName:   data["last_name"].(string),
		TempAvatar: tempAvatarURI,
	}, nil
}
