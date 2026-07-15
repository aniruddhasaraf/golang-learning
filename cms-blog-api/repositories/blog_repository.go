package repositories

import (
	"cms-blog-api/models"

	"gorm.io/gorm"
)

type BlogRepository interface {
	Create(blog *models.Blog) error
	GetAll() ([]models.Blog, error)
	GetByID(id uint) (*models.Blog, error)
	Update(blog *models.Blog) error
	Delete(id uint) error
	GetByCategory(category string) ([]models.Blog, error)
}

type blogRepository struct {
	db *gorm.DB
}

func NewBlogRepository(db *gorm.DB) BlogRepository {
	return &blogRepository{
		db: db,
	}
}

func (r *blogRepository) Create(blog *models.Blog) error {
	return r.db.Create(blog).Error
}

func (r *blogRepository) GetAll() ([]models.Blog, error) {

	var blogs []models.Blog

	err := r.db.Find(&blogs).Error

	return blogs, err
}

func (r *blogRepository) GetByID(id uint) (*models.Blog, error) {

	var blog models.Blog

	err := r.db.First(&blog, id).Error

	if err != nil {
		return nil, err
	}

	return &blog, nil
}

func (r *blogRepository) Update(blog *models.Blog) error {
	return r.db.Save(blog).Error
}

func (r *blogRepository) Delete(id uint) error {
	return r.db.Delete(&models.Blog{}, id).Error
}

func (r *blogRepository) GetByCategory(category string) ([]models.Blog, error) {

	var blogs []models.Blog

	err := r.db.Where("category = ?", category).Find(&blogs).Error

	return blogs, err
}