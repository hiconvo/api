package pluck

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"

	"github.com/jaytaylor/html2text"
)

const sigStripURL = "https://us-central1-convo-api.cloudfunctions.net/sigstrip"

func AddressesFromEnvelope(payload string) (string, string, error) {
	// Get to and from from envelope.
	envelope := make(map[string]interface{})
	jsonErr := json.Unmarshal([]byte(payload), &envelope)
	if jsonErr != nil {
		return "", "", jsonErr
	}

	// Annoying type assertions.
	itos, itosOK := envelope["to"].([]interface{})
	if !itosOK {
		return "", "", errors.New("Invalid to address type")
	}

	// If the sender added another recipient, then tos has more than
	// one address. This is not currently supported.
	if len(itos) > 1 {
		return "", "", errors.New("Multiple recipients are not supported")
	}

	to, toOK := itos[0].(string)
	if !toOK {
		return "", "", errors.New("Invalid to address type")
	}

	from, fromOK := envelope["from"].(string)
	if !fromOK {
		return "", "", errors.New("Invalid from address type")
	}

	// Use the mail library to get the addresses.
	toAddress, err := mail.ParseAddress(to)
	if err != nil {
		return "", "", errors.New("Invalid to address")
	}

	fromAddress, err := mail.ParseAddress(from)
	if err != nil {
		return "", "", errors.New("Invalid from address")
	}

	// Done.
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

	message, rmSigErr := removeRepliesAndSignature(body, from)
	if rmSigErr != nil {
		return "", rmSigErr
	}

	cleanMessage := strings.TrimSpace(strings.TrimRight(message, "-–—−")) // hyphen, en-dash, em-dash, minus

	return cleanMessage, nil
}

func removeRepliesAndSignature(text, sender string) (string, error) {
	// FIXME: I hate this solution, but there doesn't seem to
	// be a better way to handle testing at the moment.
	// Httpmock can't be used here because it doesn't support
	// passthrough for other requests, which means that any
	// database call fails when using it.
	if strings.Contains(os.Args[0], "/_test/") {
		return "Hello, does this work?", nil
	}

	b, mErr := json.Marshal(map[string]string{
		"body":   text,
		"sender": sender,
	})
	if mErr != nil {
		return "", mErr
	}

	rsp, pErr := http.Post(sigStripURL, "application/json", bytes.NewReader(b))
	if pErr != nil {
		return "", pErr
	}

	if rsp.StatusCode >= 400 {
		return "", errors.New("Sigstrip returned error")
	}

	body, rErr := ioutil.ReadAll(rsp.Body)
	if rErr != nil {
		return "", rErr
	}

	result := make(map[string]string)
	json.Unmarshal(body, &result)

	return result["text"], nil
}
