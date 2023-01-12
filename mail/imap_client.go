package mail

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/google/uuid"
)

// ImapClient a imap client
type ImapClient struct {
	Client   *client.Client
	UserName string
	Passwd   string
	Addr     string
	IsTLS    bool
}

// ImapClientInitOpt options to init an ImapClient
type ImapClientInitOpt struct {
	Addr     string
	UserName string
	Passwd   string
	IsTLS    bool
}

// NewImapClient init a imap Client
func NewImapClient(opt ImapClientInitOpt) (c *ImapClient, err error) {
	c = new(ImapClient)

	c.UserName = opt.UserName
	c.Passwd = opt.Passwd
	c.Addr = opt.Addr
	c.IsTLS = opt.IsTLS

	// try login
	if err = c.Login(); err != nil {
		return nil, err
	}

	if err = c.LogOut(); err != nil {
		return nil, err
	}

	return c, nil
}

// Login login to service
func (c *ImapClient) Login() error {
	var err error
	// Connect to server
	if c.IsTLS {
		c.Client, err = client.DialTLS(c.Addr, nil)
	} else {
		c.Client, err = client.Dial(c.Addr)
	}
	if err != nil {
		return err
	}

	return c.Client.Login(c.UserName, c.Passwd)
}

// LogOut LogOut from service
func (c *ImapClient) LogOut() error {
	err := c.Client.Logout()
	c.Client = nil
	return err
}

// GetUnReadMailIDs get all unread mails
func (c *ImapClient) GetUnReadMailIDs(mailBox string) ([]uint32, error) {
	if err := c.Login(); err != nil {
		return nil, err
	}
	defer c.LogOut()

	if len(mailBox) == 0 {
		mailBox = "INBOX"
	}

	// Select mail box
	_, err := c.Client.Select(mailBox, false)
	if err != nil {
		return nil, err
	}

	// Set search criteria
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := c.Client.Search(criteria)
	if err != nil {
		return nil, err
	}

	return ids, err
}

// Store store status
func (c *ImapClient) Store(mailBox string, mID uint32, isAdd bool, flags []interface{}) error {
	if err := c.Login(); err != nil {
		return err
	}
	defer c.LogOut()

	return c.store(mailBox, mID, isAdd, flags)
}

// store store status without login
func (c *ImapClient) store(mailBox string, mID uint32, isAdd bool, flags []interface{}) error {
	if len(mailBox) == 0 {
		mailBox = "INBOX"
	}

	// Select INBOX
	_, err := c.Client.Select(mailBox, false)
	if err != nil {
		return err
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(mID)

	var opt imap.FlagsOp
	if isAdd {
		opt = imap.AddFlags
	} else {
		opt = imap.RemoveFlags
	}

	item := imap.FormatFlagsOp(opt, true)

	return c.Client.Store(seqSet, item, flags, nil)
}

// DeleteMail delete one mail
func (c *ImapClient) DeleteMail(mailBox string, mID uint32) error {
	if err := c.Login(); err != nil {
		return err
	}
	defer c.LogOut()

	// First mark the message as deleted
	if err := c.store(mailBox, mID, true, []interface{}{imap.DeletedFlag}); err != nil {
		return err
	}

	// Then delete it
	err := c.Client.Expunge(nil)
	return err
}

// FetchMail fetch a mail
func (c *ImapClient) FetchMail(id uint32, box string, requestBody bool) (*mail.Reader, error) {
	var err error

	if err = c.Login(); err != nil {
		return nil, err
	}
	defer c.LogOut()

	if len(box) == 0 {
		box = "INBOX"
	}

	// Select mail box
	_, err = c.Client.Select(box, false)
	if err != nil {
		return nil, err
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(id)

	var section = imap.BodySectionName{}
	if !requestBody {
		section.BodyPartName.Specifier = imap.HeaderSpecifier
	}
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 1)

	go func() {
		err = c.Client.Fetch(seqSet, items, messages)
	}()

	msg := <-messages
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, errors.New("server didn't returned message")
	}

	r := msg.GetBody(&section)
	if r == nil {
		return nil, errors.New("server didn't returned message body")
	}

	// Create a new mail reader
	mr, err := mail.CreateReader(r)
	if err != nil {
		return nil, err
	}

	return mr, nil
}

// Mail save an mail data
type Mail struct {
	Client *ImapClient
	ID     uint32
	Box    string

	// header
	Date    time.Time
	Subject string
	Heads   map[string][]*mail.Address

	// body
	Content []byte

	Deleted bool
}

// GetHeadsAddressAsString convert []*net/mail.Address to string with comma separate
func (m *Mail) GetHeadsAddressAsString(head string) string {
	var addresses []string
	for address := range m.Heads[head] {
		addresses = append(addresses, m.Heads[head][address].Address)
	}
	return strings.Join(addresses, ",")
}

// GetUnReadMails get all unread mails
func (c *ImapClient) GetUnReadMails(mailBox string, limit int) ([]*Mail, error) {
	ids, err := c.GetUnReadMailIDs(mailBox)
	if err != nil {
		return nil, err
	}

	last := len(ids)
	if last > limit {
		last = limit
	}

	mails := make([]*Mail, last)
	for index, id := range ids[0:last] {
		mails[index] = &Mail{
			ID:     id,
			Client: c,
			Box:    mailBox,
		}
	}

	return mails, nil
}

// LoadHeader load Head data
func (m *Mail) LoadHeader(requestHeads []string) error {
	mr, err := m.Client.FetchMail(m.ID, m.Box, false)
	if err != nil {
		return err
	}
	defer mr.Close()

	m.Date, err = mr.Header.Date()
	if err != nil {
		return err
	}

	m.Subject, err = mr.Header.Subject()
	if err != nil {
		return err
	}

	if m.Heads == nil {
		m.Heads = make(map[string][]*mail.Address)
	}

	var v []*mail.Address
	for _, head := range requestHeads {
		if v, err = mr.Header.AddressList(head); err != nil {
			return err
		}
		m.Heads[head] = v
	}

	return nil
}

// LoadBody load body data
func (m *Mail) LoadBody() error {
	mr, err := m.Client.FetchMail(m.ID, m.Box, true)
	if err != nil {
		return err
	}
	defer mr.Close()

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		switch p.Header.(type) {
		case *mail.InlineHeader:
			// This is the message's text (can be plain-text or HTML)
			m.Content, err = io.ReadAll(p.Body)
			if err != nil {
				return err
			}
		case *mail.AttachmentHeader:
			// TODO: how to handle attachment
			// This is an attachment
			// filename, err := h.Filename()
			// if err != nil {

			// }
		}
	}

	return nil
}

// SetRead set read status
func (m *Mail) SetRead(isRead bool) error {
	return m.Client.Store(m.Box, m.ID, isRead, []interface{}{imap.SeenFlag})
}

// Delete delet this mail
func (m *Mail) Delete() error {
	if m.Deleted {
		return nil
	}
	err := m.Client.DeleteMail(m.Box, m.ID)
	if err != nil {
		return err
	}
	m.Deleted = true

	return nil
}

// GetBodyText get body text from imap.Message
func GetBodyText(m *imap.Message) ([]byte, error) {
	// Get the whole message body
	var section imap.BodySectionName

	r := m.GetBody(&section)
	if r == nil {
		return nil, nil
	}

	// Create a new mail reader
	mr, err := mail.CreateReader(r)
	if err != nil {
		return nil, err
	}
	defer mr.Close()

	var Body []byte

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		switch p.Header.(type) {
		case *mail.InlineHeader:
			// This is the message's text (can be plain-text or HTML)
			Body, err = io.ReadAll(p.Body)
			if err != nil {
				return nil, err
			}
		case *mail.AttachmentHeader:
			// TODO: how to handle attachment
			// This is an attachment
			// filename, err := h.Filename()
			// if err != nil {

			// }
		}
	}

	return Body, nil
}

// ImapAdToMailAd - convert Imap/Address to Mail/Address
func ImapAdToMailAd(addresses []*imap.Address) []*mail.Address {
	var mailAddr []*mail.Address
	for _, address := range addresses {
		// Convert imap.Address to mail.Address
		mailAddr = append(mailAddr, &mail.Address{
			Name:    address.MailboxName,
			Address: fmt.Sprintf("%s@%s", address.MailboxName, address.HostName),
		})
	}
	return mailAddr
}

// GetAddressList get Cc, From, To address list from imap.Message
func GetAddressList(m *imap.Message, requestHeads []string) (map[string][]*mail.Address, error) {
	heads := make(map[string][]*mail.Address)
	for _, head := range requestHeads {
		switch head {
		case "From":
			heads[head] = ImapAdToMailAd(m.Envelope.From)
		case "To":
			heads[head] = ImapAdToMailAd(m.Envelope.To)
		case "Cc":
			heads[head] = ImapAdToMailAd(m.Envelope.Cc)
		}
	}

	return heads, nil
}

// GetLastMailIDs get all unread mails
func (c *ImapClient) GetLastMailIDs(mailBox string, sentDate time.Time) ([]uint32, error) {
	if err := c.Login(); err != nil {
		return nil, err
	}
	defer c.LogOut()

	if len(mailBox) == 0 {
		mailBox = "INBOX"
	}

	// Select mail box
	_, err := c.Client.Select(mailBox, false)
	if err != nil {
		return nil, err
	}

	// Set search criteria
	criteria := imap.NewSearchCriteria()
	criteria.SentSince = sentDate
	ids, err := c.Client.Search(criteria)
	if err != nil {
		return nil, err
	}

	return ids, err
}

// GetLastMails get all unread mails
func (c *ImapClient) GetLastMails(mailBox string, sentDate time.Time) ([]*Mail, error) {
	ids, err := c.GetLastMailIDs(mailBox, sentDate)
	if err != nil {
		return nil, err
	}

	if err := c.Login(); err != nil {
		return nil, err
	}
	defer c.LogOut()

	mbox, err := c.Client.Select(mailBox, false)
	if err != nil {
		return nil, err
	}
	if mbox.Messages == 0 {
		return nil, nil
	}
	var emails []*imap.Message

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)

	// Get the whole message body
	var section imap.BodySectionName
	messages := make(chan *imap.Message, mbox.Messages)
	done := make(chan error, 1)
	go func() {
		done <- c.Client.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, section.FetchItem(), imap.FetchAll}, messages)
	}()

	for msg := range messages {
		emails = append(emails, msg)
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	last := len(ids)

	mails := make([]*Mail, last)
	for index, email := range emails {
		body, _ := GetBodyText(email)
		heads, _ := GetAddressList(email, []string{"From", "To", "Cc"})
		mails[index] = &Mail{
			ID:      email.SeqNum,
			Client:  c,
			Box:     mailBox,
			Content: body,
			Date:    email.Envelope.Date,
			Heads:   heads,
			Subject: email.Envelope.Subject,
		}
	}

	return mails, nil
}

func (imapClient *ImapClient) SyncEmailsWithDB(mysqlClient *MysqlClient) error {

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	doneMailBoxes := make(chan error, 1)
	go func() {
		doneMailBoxes <- imapClient.Client.List("", "*", mailboxes)
	}()

	if err := <-doneMailBoxes; err != nil {
		return err
	}

	var emails []Email
	var email Email

	for mailbox := range mailboxes {
		maxDate := mysqlClient.GetMaxDateSentFromInbox(mailbox.Name, imapClient.UserName)
		mails, err := imapClient.GetLastMails(mailbox.Name, maxDate)
		if err != nil {
			return err
		}
		for mail := range mails {
			email.Account = imapClient.UserName
			email.Body = string(mails[mail].Content)
			email.From = mails[mail].GetHeadsAddressAsString("From")
			email.To = mails[mail].GetHeadsAddressAsString("To")
			email.CC = mails[mail].GetHeadsAddressAsString("Cc")
			email.Subject = mails[mail].Subject
			email.MessageBox = mailbox.Name
			email.ID, _ = uuid.NewUUID()
			email.SentDatetime = mails[mail].Date
			emails = append(emails, email)
		}
	}
	mysqlClient.DB.Create(&emails)
	return nil
}

func GetMessages(count int) {

	mailCient, err := client.DialTLS("imap.mail.ru:993", nil)
	if err != nil {
		log.Fatal(err)
	}
	// Don't forget to logout
	defer mailCient.Logout()

	// Login
	if err := mailCient.Login("", ""); err != nil {
		log.Fatal(err)
	}
}
