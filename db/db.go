package db

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/sirupsen/logrus"
)

type Site struct {
	gorm.Model
	URL string
}

type Meta struct {
	gorm.Model
	Name  string
	Value string

	SiteID uint
	Site   Site
}

func Init() *gorm.DB {
	db, err := gorm.Open("sqlite3", "database.sqlite")
	if err != nil {
		logrus.Fatal(err)
	}
	db.CreateTable(&Meta{}, &Site{})
	db.LogMode(true)
	return db
}
