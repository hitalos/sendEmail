package main

import (
	"cmp"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	"github.com/hitalos/sendEmail"
)

var (
	smtpHost   = os.Getenv("SMTP_HOST")
	smtpPort   = os.Getenv("SMTP_PORT")
	smtpUser   = os.Getenv("SMTP_USER")
	smtpSecure = os.Getenv("SMTP_SECURE") != "false"
	password   = os.Getenv("SMTP_PASS")
)

func main() {
	from := flag.String("from", "", "sender email address")
	destinations := flag.String("dests", "", "destination email addresses (comma separated)")
	subject := flag.String("sub", "", "subject of the message")
	msgFile := flag.String("msg", "", "message file (default stdin)")
	attachments := flag.String("atts", "", "attachments to send")
	isHTML := flag.Bool("html", false, "is html message")
	isDryRun := flag.Bool("dry-run", false, "dry run")

	flag.Parse()
	if *destinations == "" || *subject == "" {
		flag.Usage()
		return
	}

	var (
		input  = os.Stdin
		sender = cmp.Or(*from, smtpUser)

		err error
	)

	if *msgFile != "" {
		input, err = os.Open(*msgFile)
		if err != nil {
			print("Error opening message file: " + err.Error())
			os.Exit(1)
		}
	}

	msg, err := io.ReadAll(input)
	if err != nil {
		print("Error reading from input: " + err.Error())
		os.Exit(1)
	}

	m := sendEmail.NewMessage().
		SetFrom(sender).
		SetTo(*destinations).
		SetSubject(*subject)

	if *isHTML {
		m.SetHtml(fmt.Sprintf("<p>%s</p>", strings.Join(strings.Split(string(msg), "\n"), "</p><p>")))
	} else {
		m.SetPlainText(string(msg))
	}

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
		smtpPort = "465"
	}

	var (
		err        error
		host       = fmt.Sprintf("%s:%s", smtpHost, smtpPort)
		smtpClient *smtp.Client
	)

	tlsConfig := &tls.Config{ServerName: smtpHost, InsecureSkipVerify: smtpSecure}
	switch smtpPort {
	case "25", "587":
		{
			smtpClient, err = smtp.Dial(host)
			if err != nil {
				return nil, err
			}

			if err = smtpClient.StartTLS(tlsConfig); err != nil {
				return nil, err
			}
		}
	default:
		{
			tlsConfig := tlsConfig
			conn, err := tls.Dial("tcp4", host, tlsConfig)
			if err != nil {
				return nil, err
			}
			smtpClient, err = smtp.NewClient(conn, smtpHost)
			if err != nil {
				return nil, err
			}
		}
	}

	if smtpUser != "" && password != "" {
		auth := smtp.PlainAuth("", smtpUser, password, smtpHost)
		if err = smtpClient.Auth(auth); err != nil {
			return nil, err
		}
	}

	return smtpClient, nil
}
