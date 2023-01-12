package mail

import (
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MysqlClient a imap client
type MysqlClient struct {
	DB       *gorm.DB
	DBDriver string
	DBUser   string
	DBPass   string
	DBName   string
	DBHost   string
	DBPort   string
	DBParams string
}

// MysqlClientInitOpt options to init an MysqlClient
type MysqlClientInitOpt struct {
	DBDriver string
	DBUser   string
	DBPass   string
	DBName   string
	DBHost   string
	DBPort   string
	DBParams string
}

// Mail DTO
type Email struct {
	gorm.Model
	ID           uuid.UUID `gorm:"type:uuid;default:UUID();primary_key;"`
	From         string
	To           string
	CC           string
	Subject      string
	SentDatetime time.Time
	Body         string
	MessageUUID  uuid.UUID `gorm:"type:uuid"`
	MessageBox   string
	Tags         string
	IsRead       bool `gorm:"type:bool;default:false"`
	Account      string
}

// NewMysqlClient init a imap Client
func NewMysqlClient(opt MysqlClientInitOpt) (c *MysqlClient, err error) {
	c = new(MysqlClient)

	c.DBDriver = opt.DBDriver
	c.DBUser = opt.DBUser
	c.DBPass = opt.DBPass
	c.DBName = opt.DBName
	c.DBHost = opt.DBHost
	c.DBPort = opt.DBPort
	c.DBParams = opt.DBParams

	dsn := c.DBUser + ":" + c.DBPass + "@tcp(" + c.DBHost + ":" + c.DBPort + ")/" + c.DBName + "?" + c.DBParams
	c.DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err.Error())
	}
	return c, nil
}

func (c *MysqlClient) GetMaxDateSentFromInbox(mailbox string, account string) time.Time {
	var maxDate time.Time
	c.DB.Table("emails").Select("MAX(sent_datetime)").Where("message_box = ? AND account = ?", mailbox, account).Scan(&maxDate)
	return maxDate
}

// BeforeCreate will set a UUID rather than numeric ID.
func (email *Email) BeforeCreate(scope *gorm.DB) error {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return err
	}
	scope.Statement.SetColumn("ID", uuid)
	return nil
}
