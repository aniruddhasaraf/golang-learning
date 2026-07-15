package models

import "time"

type Blog struct {
	ID uint `gorm:"primaryKey" json:"id"`

	Title string `gorm:"type:varchar(255);not null" json:"title" binding:"required"`

	Content string `gorm:"type:text;not null" json:"content" binding:"required"`

	Author string `gorm:"type:varchar(100);not null" json:"author" binding:"required"`

	Category string `gorm:"type:varchar(100);not null" json:"category" binding:"required"`

	Status string `gorm:"type:varchar(20);not null" json:"status" binding:"required"`

	CreatedAt time.Time `json:"created_at"`

	UpdatedAt time.Time `json:"updated_at"`
}