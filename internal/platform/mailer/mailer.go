package mailer

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"sakura-happy-cottage/internal/domain"
)

type Sender interface {
	Send(settings domain.SystemSettings, to, subject, htmlBody string) error
}

type SMTP struct{}

func (SMTP) Send(settings domain.SystemSettings, to, subject, htmlBody string) error {
	fromAddress, err := parseAddress(settings.SMTPFromEmail)
	if err != nil {
		return fmt.Errorf("invalid SMTP from address: %w", err)
	}
	toAddress, err := parseAddress(to)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}
	if containsNewline(settings.SMTPFromName) || containsNewline(subject) {
		return fmt.Errorf("mail header contains a newline")
	}
	address := net.JoinHostPort(settings.SMTPHost, fmt.Sprintf("%d", settings.SMTPPort))
	var connection net.Conn
	if settings.SMTPEncryption == "tls" {
		connection, err = tls.Dial("tcp", address, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: settings.SMTPHost})
	} else {
		connection, err = net.DialTimeout("tcp", address, 15*time.Second)
	}
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
	client, err := smtp.NewClient(connection, settings.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if settings.SMTPEncryption == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP 服务器不支持 STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: settings.SMTPHost}); err != nil {
			return err
		}
	}
	if settings.SMTPUsername != "" {
		if err := client.Auth(smtp.PlainAuth("", settings.SMTPUsername, settings.SMTPPassword, settings.SMTPHost)); err != nil {
			return err
		}
	}
	if err := client.Mail(fromAddress.Address); err != nil {
		return err
	}
	if err := client.Rcpt(toAddress.Address); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	buffer := bufio.NewWriter(writer)
	fromAddress.Name = strings.TrimSpace(settings.SMTPFromName)
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", fromAddress.String(), toAddress.String(), mime.QEncoding.Encode("UTF-8", subject), htmlBody)
	if _, err := buffer.WriteString(message); err != nil {
		return err
	}
	if err := buffer.Flush(); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func parseAddress(value string) (*mail.Address, error) {
	value = strings.TrimSpace(value)
	if containsNewline(value) {
		return nil, fmt.Errorf("address contains a newline")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return nil, fmt.Errorf("address must contain one plain mailbox")
	}
	return address, nil
}

func containsNewline(value string) bool { return strings.ContainsAny(value, "\r\n") }
