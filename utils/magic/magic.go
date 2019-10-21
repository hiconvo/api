package magic

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/utils/secrets"
)

var secret = secrets.Get("APP_SECRET", "")

func NewLink(k *datastore.Key, salt, action string) string {
	// Get time and convert to epoc string
	ts := time.Now().Unix()
	sts := strconv.FormatInt(ts, 10)
	b64ts := base64.URLEncoding.EncodeToString([]byte(sts))

	// Get url-safe key
	kenc := k.Encode()

	return fmt.Sprintf("https://app.hiconvo.com/%s/%s/%s/%s",
		action, kenc, b64ts, getSignature(kenc, b64ts, salt))
}

func Verify(kenc, b64ts, salt, sig string) bool {
	return sig == getSignature(kenc, b64ts, salt)
}

func GetTimeFromB64(b64ts string) (time.Time, error) {
	byteTime, err := base64.URLEncoding.DecodeString(b64ts)
	if err != nil {
		return time.Now(), err
	}

	stringTime := bytes.NewBuffer(byteTime).String()

	intTime, err := strconv.Atoi(stringTime)
	if err != nil {
		return time.Now(), err
	}

	timestamp := time.Unix(int64(intTime), 0)

	return timestamp, nil
}

func getSignature(uid, b64ts, salt string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(uid + b64ts + salt))
	sha := hex.EncodeToString(h.Sum(nil))
	return sha
}
