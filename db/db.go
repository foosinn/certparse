package db

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/sirupsen/logrus"
)

type Site struct {
	gorm.Model
	Name string
	URL  string `gorm:unique;index`

	Categories []*Category `gorm:"many2many:site_categories"`
	Tags       []*Tag      `gorm:"many2many:site_tags"`
}

type Category struct {
	gorm.Model
	Name  string
	Sites []*Site `gorm:"many2many:sites_categories"`
}

type Tag struct {
	gorm.Model
	Name  string
	Sites []*Site `gorm:"many2many:sites_tags"`
}

var db *gorm.DB

func Init() *gorm.DB {
	db, err := gorm.Open("sqlite3", "database.sqlite")
	if err != nil {
		logrus.Fatal(err)
	}
	db.CreateTable(&Category{}, &Tag{}, &Site{})
	return db
}
