package main

import (
	"log"
	"os"

	"cms-blog-api/config"
	"cms-blog-api/controllers"
	"cms-blog-api/repositories"
	"cms-blog-api/routes"
	"cms-blog-api/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatal("❌ Error loading .env")
	}

	log.Println("✅ Environment Loaded")

	config.ConnectDatabase()
	config.ConnectRedis()

	// Dependency Injection
	blogRepo := repositories.NewBlogRepository(config.DB)
	blogService := services.NewBlogService(blogRepo)
	blogController := controllers.NewBlogController(blogService)

	router := gin.Default()

	routes.SetupRoutes(router, blogController)

	port := os.Getenv("PORT")

	log.Println("🚀 Server Running On Port :", port)

	router.Run(":" + port)
}