package routes

import (
	"cms-blog-api/controllers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, blogController *controllers.BlogController) {

	router.POST("/blogs", blogController.CreateBlog)

	router.GET("/blogs", blogController.GetBlogs)

	router.GET("/blogs/:id", blogController.GetBlogByID)

	router.PUT("/blogs/:id", blogController.UpdateBlog)

	router.DELETE("/blogs/:id", blogController.DeleteBlog)

	router.GET("/blogs/category/:category", blogController.GetBlogsByCategory)
}