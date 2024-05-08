package models

import "gorm.io/gorm"

type Item struct {
    gorm.Model
    Name        string
    Composition string
    Image       string
    Price       float64
	TypeId      uint 
    Count       int
}
