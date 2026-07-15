package services

import (
	"cms-blog-api/models"
	"cms-blog-api/repositories"
)

type BlogService interface {
	CreateBlog(blog *models.Blog) error
	GetBlogs() ([]models.Blog, error)
	GetBlog(id uint) (*models.Blog, error)
	UpdateBlog(blog *models.Blog) error
	DeleteBlog(id uint) error
	GetBlogsByCategory(category string) ([]models.Blog, error)
}

type blogService struct {
	repo repositories.BlogRepository
}

func NewBlogService(repo repositories.BlogRepository) BlogService {
	return &blogService{
		repo: repo,
	}
}

func (s *blogService) CreateBlog(blog *models.Blog) error {
	return s.repo.Create(blog)
}

func (s *blogService) GetBlogs() ([]models.Blog, error) {
	return s.repo.GetAll()
}

func (s *blogService) GetBlog(id uint) (*models.Blog, error) {
	return s.repo.GetByID(id)
}

func (s *blogService) UpdateBlog(blog *models.Blog) error {
	return s.repo.Update(blog)
}

func (s *blogService) DeleteBlog(id uint) error {
	return s.repo.Delete(id)
}

func (s *blogService) GetBlogsByCategory(category string) ([]models.Blog, error) {
	return s.repo.GetByCategory(category)
}