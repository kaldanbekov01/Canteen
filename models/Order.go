package models

import "gorm.io/gorm"

type Order struct {
    gorm.Model
    BasketID    uint 
	Status      string
}
