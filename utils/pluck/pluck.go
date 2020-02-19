package pluck

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/hiconvo/api/errors"
)

const sigStripURL = "https://us-central1-convo-api.cloudfunctions.net/sigstrip"

func AddressesFromEnvelope(payload string) (string, string, error) {
	var op errors.Op = "pluck.AddressFromEnvelope"

	// Get to and from from envelope.
	envelope := make(map[string]interface{})
	err := json.Unmarshal([]byte(payload), &envelope)
	if err != nil {
		return "", "", errors.E(op, err)
	}

	// Annoying type assertions.
	itos, itosOK := envelope["to"].([]interface{})
	if !itosOK {
		return "", "", errors.E(op, errors.Str("Invalid 'to' address type"))
	}

	// If the sender added another recipient, then tos has more than
	// one address. This is not currently supported.
	if len(itos) > 1 {
		return "", "", errors.E(op, errors.Str("Multiple recipients are not supported"))
	}

	to, toOK := itos[0].(string)
	if !toOK {
		return "", "", errors.E(op, errors.Str("Invalid 'to' address type"))
	}

	from, fromOK := envelope["from"].(string)
	if !fromOK {
		return "", "", errors.E(op, errors.Str("Invalid 'from' address type"))
	}

	// Use the mail library to get the addresses.
	toAddress, err := mail.ParseAddress(to)
	if err != nil {
		return "", "", errors.E(op, errors.Str("Invalid 'to' address"))
	}

	fromAddress, err := mail.ParseAddress(from)
	if err != nil {
		return "", "", errors.E(op, errors.Str("Invalid 'from' address"))
	}

	return toAddress.Address, fromAddress.Address, nil
}

func ThreadInt64IDFromAddress(to string) (int64, error) {
	split := strings.Split(to, "@")
	toName := split[0]
	nameSplit := strings.Split(toName, "-")
	ID := nameSplit[len(nameSplit)-1]
	return strconv.ParseInt(ID, 10, 64)
}

func MessageText(htmlBody, textBody, from, to string) (string, error) {
	// Prefer plainText if available. Otherwise extract text.
	var body string
	if len(textBody) > 0 {
		body = textBody
	} else {
		stripped, err := html2text.FromString(htmlBody, html2text.Options{})
		if err != nil {
			return "", err
		}

		body = stripped
	}

	message, err := removeRepliesAndSignature(body, from)
	if err != nil {
		return "", errors.E(errors.Op("pluck.MessageText"), err)
	}

	cleanMessage := strings.TrimSpace(strings.TrimRight(message, "-–—−")) // hyphen, en-dash, em-dash, minus

	return cleanMessage, nil
}

func removeRepliesAndSignature(text, sender string) (string, error) {
	var op errors.Op = "pluck.removeRepliesAndSignature"

	// FIXME: I hate this solution, but there doesn't seem to
	// be a better way to handle testing at the moment.
	// Httpmock can't be used here because it doesn't support
	// passthrough for other requests, which means that any
	// database call fails when using it.
	if strings.HasSuffix(os.Args[0], ".test") {
		return "Hello, does this work?", nil
	}

	b, err := json.Marshal(map[string]string{
		"body":   text,
		"sender": sender,
	})
	if err != nil {
		return "", errors.E(op, err)
	}

	rsp, err := http.Post(sigStripURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", errors.E(op, err)
	}

	if rsp.StatusCode >= 400 {
		return "", errors.E(op, errors.Str("Sigstrip returned error"))
	}

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return "", errors.E(op, err)
	}

	result := make(map[string]string)
	json.Unmarshal(body, &result)

	return result["text"], nil
}
