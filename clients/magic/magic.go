package magic

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/errors"
)

type Client interface {
	NewLink(k *datastore.Key, salt, action string) string
	Verify(kenc, b64ts, salt, sig string) error
}

type clientImpl struct {
	secret string
}

func NewClient(secret string) Client {
	return &clientImpl{secret}
}

func (c *clientImpl) NewLink(k *datastore.Key, salt, action string) string {
	// Get time and convert to epoc string
	ts := time.Now().Unix()
	sts := strconv.FormatInt(ts, 10)
	b64ts := base64.URLEncoding.EncodeToString([]byte(sts))

	// Get url-safe key
	kenc := k.Encode()

	return fmt.Sprintf("https://app.convo.events/%s/%s/%s/%s",
		action, kenc, b64ts, c.getSignature(kenc, b64ts, salt))
}

func (c *clientImpl) Verify(kenc, b64ts, salt, sig string) error {
	if sig == c.getSignature(kenc, b64ts, salt) {
		return nil
	}

	return errors.E(errors.Op("magic.Verify"), http.StatusUnauthorized, errors.Str("InvalidSignature"))
}

func (c *clientImpl) getSignature(uid, b64ts, salt string) string {
	h := hmac.New(sha256.New, []byte(c.secret))

	if _, err := h.Write([]byte(uid + b64ts + salt)); err != nil {
		panic(errors.E(errors.Opf("getSignature(uid=%s, b64ts=%s, salt=%s)", uid, b64ts, salt), err))
	}

	sha := hex.EncodeToString(h.Sum(nil))

	return sha
}

func GetTimeFromB64(b64ts string) (time.Time, error) {
	op := errors.Op("magic.GetTimeFromB64")

	byteTime, err := base64.URLEncoding.DecodeString(b64ts)
	if err != nil {
		return time.Now(), errors.E(op, http.StatusBadRequest, err)
	}

	stringTime := bytes.NewBuffer(byteTime).String()

	intTime, err := strconv.Atoi(stringTime)
	if err != nil {
		return time.Now(), errors.E(op, http.StatusBadRequest, err)
	}

	timestamp := time.Unix(int64(intTime), 0)

	return timestamp, nil
}

func TooOld(b64ts string, days ...int) error {
	op := errors.Op("magic.TooOld")

	ts, err := GetTimeFromB64(b64ts)
	if err != nil {
		return errors.E(op, http.StatusUnauthorized, err)
	}

	expiration := 24
	if len(days) > 0 {
		expiration = days[0] * expiration
	}

	diff := time.Since(ts)
	if diff.Hours() > float64(expiration) {
		return errors.E(op, http.StatusUnauthorized, errors.Str("TooOld"))
	}

	return nil
}
