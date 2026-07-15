package config

import (
	"fmt"
	"log"
	"os"

	"cms-blog-api/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatal("❌ Failed to connect MySQL :", err)
	}

	DB = db

	err = DB.AutoMigrate(&models.Blog{})

	if err != nil {
		log.Fatal("❌ Migration Failed :", err)
	}

	log.Println("✅ MySQL Connected")
	log.Println("✅ Blog Table Migrated")
}