package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	"github.com/hitalos/sendEmail"
)

var (
	smtpHost = os.Getenv("SMTP_HOST")
	smtpPort = os.Getenv("SMTP_PORT")
	sender   = os.Getenv("SMTP_USER")
	password = os.Getenv("SMTP_PASS")
)

func main() {
	from := flag.String("from", sender, "sender email address")
	destinations := flag.String("dests", "", "destination email addresses (comma separated)")
	msg := flag.String("msg", "", "message to send")
	subject := flag.String("sub", "", "subject of the message")
	attachments := flag.String("atts", "", "attachments to send")
	isDryRun := flag.Bool("dry-run", false, "dry run")

	flag.Parse()
	if *destinations == "" || *msg == "" || *subject == "" {
		flag.Usage()
		return
	}

	m := sendEmail.NewMessage().
		SetFrom(*from).
		SetTo(*destinations).
		SetSubject(*subject).
		SetPlainText(*msg).
		SetHtml("<p>" + strings.Join(strings.Split(*msg, "\n"), "</p><p>") + "</p>")

	if *attachments != "" {
		for _, a := range strings.Split(*attachments, ",") {
			m.AddAttachment(a)
		}
	}

	if *isDryRun {
		if err := m.Write(os.Stdout); err != nil {
			print(err.Error())
		}
		return
	}

	smtpClient, err := connectAndAuth()
	if err != nil {
		print(err.Error())
		return
	}
	defer func() {
		if err = smtpClient.Quit(); err != nil {
			print(err.Error())
		}
	}()

	if err = m.Send(smtpClient); err != nil {
		print(err.Error())
		return
	}
}

func connectAndAuth() (*smtp.Client, error) {
	if _, err := strconv.Atoi(smtpPort); err != nil {
		smtpPort = "587"
	}

	smtpClient, err := smtp.Dial(fmt.Sprintf("%s:%s", smtpHost, smtpPort))
	if err != nil {
		return nil, err
	}

	if err = smtpClient.StartTLS(&tls.Config{ServerName: smtpHost}); err != nil {
		return nil, err
	}

	auth := smtp.PlainAuth("", sender, password, smtpHost)
	if err = smtpClient.Auth(auth); err != nil {
		return nil, err
	}

	return smtpClient, nil
}
