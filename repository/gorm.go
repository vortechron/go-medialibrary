package repository

import (
	"context"
	"fmt"

	"github.com/vortechron/go-medialibrary/medialibrary"
	"github.com/vortechron/go-medialibrary/models"
	"gorm.io/gorm"
)


type GormMediaRepository struct {
	db *gorm.DB
}


func NewGormMediaRepository(db *gorm.DB) *GormMediaRepository {
	return &GormMediaRepository{
		db: db,
	}
}


func (r *GormMediaRepository) AutoMigrate() error {
	err := r.db.AutoMigrate(&models.Media{})
	if err != nil {
		return fmt.Errorf("failed to migrate media model: %w", err)
	}
	return nil
}


func (r *GormMediaRepository) Save(ctx context.Context, media *models.Media) error {
	tx := r.db.WithContext(ctx)


	if media.ID == 0 {

		if err := tx.Create(media).Error; err != nil {
			return fmt.Errorf("failed to create media record: %w", err)
		}
	} else {

		if err := tx.Save(media).Error; err != nil {
			return fmt.Errorf("failed to update media record: %w", err)
		}
	}

	return nil
}


func (r *GormMediaRepository) FindByID(ctx context.Context, id uint64) (*models.Media, error) {
	var media models.Media

	tx := r.db.WithContext(ctx)
	if err := tx.Where("id = ?", id).First(&media).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find media by ID: %w", err)
	}

	return &media, nil
}


func (r *GormMediaRepository) Delete(ctx context.Context, media *models.Media) error {
	tx := r.db.WithContext(ctx)
	if err := tx.Delete(media).Error; err != nil {
		return fmt.Errorf("failed to delete media: %w", err)
	}

	return nil
}


func (r *GormMediaRepository) FindByModelTypeAndID(ctx context.Context, modelType string, modelID uint64) ([]*models.Media, error) {
	var media []*models.Media

	tx := r.db.WithContext(ctx)
	if err := tx.Where("model_type = ? AND model_id = ?", modelType, modelID).Find(&media).Error; err != nil {
		return nil, fmt.Errorf("failed to find media by model: %w", err)
	}

	return media, nil
}


func (r *GormMediaRepository) FindByCollection(ctx context.Context, collection string) ([]*models.Media, error) {
	var media []*models.Media

	tx := r.db.WithContext(ctx)
	if err := tx.Where("collection_name = ?", collection).Find(&media).Error; err != nil {
		return nil, fmt.Errorf("failed to find media by collection: %w", err)
	}

	return media, nil
}


func (r *GormMediaRepository) FindByModelAndCollection(ctx context.Context, modelType string, modelID uint64, collection string) ([]*models.Media, error) {
	var media []*models.Media

	tx := r.db.WithContext(ctx)
	if err := tx.Where("model_type = ? AND model_id = ? AND collection_name = ?", modelType, modelID, collection).Find(&media).Error; err != nil {
		return nil, fmt.Errorf("failed to find media by model and collection: %w", err)
	}

	return media, nil
}


var _ medialibrary.MediaRepository = (*GormMediaRepository)(nil)
