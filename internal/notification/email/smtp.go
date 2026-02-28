package email

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/openrsvp/openrsvp/internal/notification"
)

// SMTPProvider sends emails via SMTP.
type SMTPProvider struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// NewSMTPProvider creates a new SMTPProvider with the given SMTP configuration.
func NewSMTPProvider(host, port, username, password, from string) *SMTPProvider {
	return &SMTPProvider{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// Name returns the provider identifier.
func (p *SMTPProvider) Name() string {
	return "smtp"
}

// Channel returns which channel this provider serves.
func (p *SMTPProvider) Channel() notification.Channel {
	return notification.ChannelEmail
}

// Send composes a proper MIME email (multipart/alternative with plain text and
// HTML) and delivers it via SMTP.
func (p *SMTPProvider) Send(ctx context.Context, msg *notification.Message) error {
	addr := net.JoinHostPort(p.host, p.port)

	// Build multipart/alternative MIME message.
	boundary := fmt.Sprintf("==OpenRSVP==%d==", time.Now().UnixNano())

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", p.from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", msg.To))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject)))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z)))
	buf.WriteString("\r\n")

	// Plain text part.
	plain := msg.Plain
	if plain == "" {
		plain = msg.Body
	}
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	qpw := quotedprintable.NewWriter(&buf)
	qpw.Write([]byte(plain))
	qpw.Close()
	buf.WriteString("\r\n")

	// HTML part.
	if msg.Body != "" && msg.Body != plain {
		buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		buf.WriteString("\r\n")
		qpw = quotedprintable.NewWriter(&buf)
		qpw.Write([]byte(msg.Body))
		qpw.Close()
		buf.WriteString("\r\n")
	}

	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	// Send via SMTP.
	var auth smtp.Auth
	if p.username != "" && p.password != "" {
		auth = smtp.PlainAuth("", p.username, p.password, p.host)
	}

	to := []string{msg.To}
	if err := smtp.SendMail(addr, auth, p.from, to, buf.Bytes()); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}

	return nil
}

// SendBatch delivers multiple notifications by iterating and sending each
// one individually.
func (p *SMTPProvider) SendBatch(ctx context.Context, msgs []*notification.Message) []error {
	errs := make([]error, len(msgs))
	for i, msg := range msgs {
		errs[i] = p.Send(ctx, msg)
	}
	return errs
}

// HealthCheck dials the SMTP server to verify connectivity.
func (p *SMTPProvider) HealthCheck(ctx context.Context) error {
	addr := net.JoinHostPort(p.host, p.port)

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp health check dial: %w", err)
	}
	defer conn.Close()

	// Attempt SMTP handshake.
	host := p.host
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp health check handshake: %w", err)
	}
	defer client.Close()

	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp health check hello: %w", err)
	}

	return client.Quit()
}
