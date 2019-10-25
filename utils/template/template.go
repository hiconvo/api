package template

// Message is a renderable message. It is always a constituent of a
// Thread. The Body field accepts markdown. XML is not allowed.
type Message struct {
	renderable
	Body   string
	Name   string
	FromID string
	ToID   string
}

// Thread is a representation of a renderable email thread.
type Thread struct {
	renderable
	Subject  string
	Messages []Message
	Preview  string
}

// Event is a representation of a renderable email event.
type Event struct {
	renderable
	Name        string
	Address     string
	Time        string
	Description string
	Preview     string
	FromName    string
	MagicLink   string
}

// Digest is a representation of a renderable email digest.
type Digest struct {
	renderable
	Items   []Thread
	Preview string
	Events  []Event
}

// RenderThread returns a rendered thread email.
func RenderThread(t Thread) (string, error) {
	for i := range t.Messages {
		t.Messages[i].RenderMarkdown(t.Messages[i].Body)
	}

	return t.RenderHTML("thread.html", t)
}

// RenderEvent returns a rendered event invitation email.
func RenderEvent(e Event) (string, error) {
	e.RenderMarkdown(e.Description)

	return e.RenderHTML("event.html", e)
}

// RenderCancellation returns a rendered event cancellation email.
func RenderCancellation(e Event) (string, error) {
	return e.RenderHTML("cancellation.html", e)
}

// RenderDigest returns a rendered digest email.
func RenderDigest(d Digest) (string, error) {
	for i := range d.Items {
		for j := range d.Items[i].Messages {
			d.Items[i].Messages[j].RenderMarkdown(d.Items[i].Messages[j].Body)
		}
	}

	d.Preview = "You have notifications on Convo."

	return d.RenderHTML("digest.html", d)
}
