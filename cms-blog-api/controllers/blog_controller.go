package controllers

import (
	"net/http"
	"strconv"

	"cms-blog-api/models"
	"cms-blog-api/services"

	"github.com/gin-gonic/gin"
)

type BlogController struct {
	service services.BlogService
}

func NewBlogController(service services.BlogService) *BlogController {
	return &BlogController{
		service: service,
	}
}

// POST /blogs
func (bc *BlogController) CreateBlog(c *gin.Context) {

	var blog models.Blog

	if err := c.ShouldBindJSON(&blog); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := bc.service.CreateBlog(&blog); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Blog created successfully",
		"data":    blog,
	})
}

// GET /blogs
func (bc *BlogController) GetBlogs(c *gin.Context) {

	blogs, err := bc.service.GetBlogs()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    blogs,
	})
}

// GET /blogs/:id
func (bc *BlogController) GetBlogByID(c *gin.Context) {

	id, _ := strconv.Atoi(c.Param("id"))

	blog, err := bc.service.GetBlog(uint(id))

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Blog not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    blog,
	})
}

// PUT /blogs/:id
func (bc *BlogController) UpdateBlog(c *gin.Context) {

	id, _ := strconv.Atoi(c.Param("id"))

	blog, err := bc.service.GetBlog(uint(id))

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Blog not found",
		})
		return
	}

	if err := c.ShouldBindJSON(blog); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := bc.service.UpdateBlog(blog); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Blog updated successfully",
		"data":    blog,
	})
}

// DELETE /blogs/:id
func (bc *BlogController) DeleteBlog(c *gin.Context) {

	id, _ := strconv.Atoi(c.Param("id"))

	if err := bc.service.DeleteBlog(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Blog deleted successfully",
	})
}

// GET /blogs/category/:category
func (bc *BlogController) GetBlogsByCategory(c *gin.Context) {

	category := c.Param("category")

	blogs, err := bc.service.GetBlogsByCategory(category)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    blogs,
	})
}