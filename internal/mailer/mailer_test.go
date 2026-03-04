package mailer

import (
	"errors"
	"testing"

	"github.com/go-mail/mail/v2"
	"github.com/usmanzaheer1995/greenlight/internal/assert"
)

type mockDialer struct {
	callCount   int
	errSequence []error
}

func (m *mockDialer) DialAndSend(msgs ...*mail.Message) error {
	i := m.callCount
	if i >= len(m.errSequence) {
		i = len(m.errSequence) - 1
	}
	m.callCount++
	return m.errSequence[i]
}

func newTestMailer(d dialer) Mailer {
	return Mailer{dialer: d, sender: "test@example.com", retryDelay: 0}
}

func TestMailer_Send_HappyPath(t *testing.T) {
	mock := &mockDialer{errSequence: []error{nil}}
	m := newTestMailer(mock)

	err := m.Send("recipient@example.com", "user_welcome.tmpl", map[string]any{
		"userName": "John",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	assert.Equal(t, mock.callCount, 1)
}

func TestMailer_Send_RetriesAndSucceeds(t *testing.T) {
	sendErr := errors.New("temporary smtp error")
	mock := &mockDialer{errSequence: []error{sendErr, sendErr, nil}}
	m := newTestMailer(mock)

	err := m.Send("recipient@example.com", "user_welcome.tmpl", map[string]any{
		"userName": "John",
	})
	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}

	assert.Equal(t, mock.callCount, 3)
}

func TestMailer_Send_AllRetriesExhausted(t *testing.T) {
	sendErr := errors.New("permanent smtp error")
	mock := &mockDialer{errSequence: []error{sendErr}}
	m := newTestMailer(mock)

	err := m.Send("recipient@example.com", "user_welcome.tmpl", map[string]any{
		"userName": "John",
	})
	if err == nil {
		t.Fatal("expected error after all retries, got nil")
	}

	assert.Equal(t, mock.callCount, 3)
}

func TestMailer_Send_BadTemplateName(t *testing.T) {
	mock := &mockDialer{errSequence: []error{nil}}
	m := newTestMailer(mock)

	err := m.Send("recipient@example.com", "nonexistent.tmpl", nil)
	if err == nil {
		t.Fatal("expected error for bad template name, got nil")
	}

	assert.Equal(t, mock.callCount, 0)
}
