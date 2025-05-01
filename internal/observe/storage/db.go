package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
}

func NewDB(host string, port uint, username string, password string, name string) *DB {
	dsn := compileDSN(host, port, username, password, name)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err.Error())
	}
	db.AutoMigrate(&Quantity{}, &Data{})
	return &DB{DB: db}
}

func compileDSN(host string, port uint, username string, password string, name string) string {
	dsnKeyValues := map[string]string{
		"host":     host,
		"port":     strconv.Itoa(int(port)),
		"user":     username,
		"password": password,
		"dbname":   name,
		"sslmode":  "disable",
	}
	dsnPairs := make([]string, 0, len(dsnKeyValues))
	for key, value := range dsnKeyValues {
		dsnPairs = append(dsnPairs, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(dsnPairs, " ")
}

type Quantity struct {
	ID    uint
	Name  string `gorm:"unique; not null"`
	Unit  string `gorm:"not null"`
	Datas []Data
}

type Data struct {
	ID         uint
	Timestamp  time.Time `gorm:"type:timestamptz(0); not null"`
	QuantityID uint      `gorm:"not null"`
	Value      float32   `gorm:"type:real; not null"`
}
