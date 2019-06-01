package title

import (
	"github.com/jinzhu/gorm"

	database "github.com/foosinn/certparse/db"
)

type (
	TitleInfo struct {
		Url     string
		TagVals map[string][]string
		Err     error
	}
)

func (ti *TitleInfo) Store(db *gorm.DB) {
	site := database.Site{}
	db.FirstOrInit(&site, database.Site{URL: ti.Url})
	db.Save(&site)

	for name, vals := range ti.TagVals {
		for _, val := range vals {
			meta := database.Meta{Name: name, Value: val, Site: site}
			db.Create(&meta)
			db.Save(&meta)
		}
	}
}
