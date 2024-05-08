package models

import "gorm.io/gorm"

type Basket struct {
	gorm.Model
	ItemID    uint `gorm:"foreignKey:ID"`
	UserID    uint `gorm:"foreignKey:ID"`
	Quantity  int
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
