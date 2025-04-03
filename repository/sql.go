package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/vortechron/go-medialibrary/medialibrary"
	"github.com/vortechron/go-medialibrary/models"
)

// SQLMediaRepository implements the MediaRepository interface using *sql.DB
type SQLMediaRepository struct {
	db *sql.DB
}

// NewSQLMediaRepository creates a new SQLMediaRepository instance
func NewSQLMediaRepository(db *sql.DB) *SQLMediaRepository {
	return &SQLMediaRepository{
		db: db,
	}
}

// CreateTablesIfNotExist creates the necessary tables if they don't exist
func (r *SQLMediaRepository) CreateTablesIfNotExist(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS media (
		id SERIAL PRIMARY KEY,
		model_type VARCHAR(255),
		model_id BIGINT,
		uuid VARCHAR(36) UNIQUE,
		collection_name VARCHAR(255),
		name VARCHAR(255),
		file_name VARCHAR(255),
		mime_type VARCHAR(255),
		disk VARCHAR(255),
		conversions_disk VARCHAR(255),
		size BIGINT,
		manipulations JSON,
		custom_properties JSON,
		generated_conversions JSON,
		responsive_images JSON,
		order_column INT,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		INDEX idx_model (model_type, model_id)
	)
	`

	// Note: The INDEX part might need to be adjusted based on your specific database (MySQL, PostgreSQL, etc.)
	// as the syntax can vary

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create media table: %w", err)
	}

	return nil
}

// scanMedia scans a row into a Media struct
func scanMedia(row *sql.Row) (*models.Media, error) {
	var media models.Media
	var uuidStr string
	var createdAt, updatedAt time.Time
	var manipulations, customProperties, generatedConversions, responsiveImages []byte
	var orderColumn sql.NullInt32

	err := row.Scan(
		&media.ID,
		&media.ModelType,
		&media.ModelID,
		&uuidStr,
		&media.CollectionName,
		&media.Name,
		&media.FileName,
		&media.MimeType,
		&media.Disk,
		&media.ConversionsDisk,
		&media.Size,
		&manipulations,
		&customProperties,
		&generatedConversions,
		&responsiveImages,
		&orderColumn,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Parse UUID
	parsed, err := uuid.FromString(uuidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID string: %w", err)
	}
	media.UUID = &parsed

	// Set JSON fields
	media.Manipulations = json.RawMessage(manipulations)
	media.CustomProperties = json.RawMessage(customProperties)
	media.GeneratedConversions = json.RawMessage(generatedConversions)
	media.ResponsiveImages = json.RawMessage(responsiveImages)

	// Handle nullable order column
	if orderColumn.Valid {
		orderColumnInt := int(orderColumn.Int32)
		media.OrderColumn = &orderColumnInt
	}

	media.CreatedAt = createdAt
	media.UpdatedAt = updatedAt

	return &media, nil
}

// scanMediaList scans rows into a slice of Media pointers
func scanMediaList(rows *sql.Rows) ([]*models.Media, error) {
	var mediaList []*models.Media

	for rows.Next() {
		var media models.Media
		var uuidStr string
		var createdAt, updatedAt time.Time
		var manipulations, customProperties, generatedConversions, responsiveImages []byte
		var orderColumn sql.NullInt32

		err := rows.Scan(
			&media.ID,
			&media.ModelType,
			&media.ModelID,
			&uuidStr,
			&media.CollectionName,
			&media.Name,
			&media.FileName,
			&media.MimeType,
			&media.Disk,
			&media.ConversionsDisk,
			&media.Size,
			&manipulations,
			&customProperties,
			&generatedConversions,
			&responsiveImages,
			&orderColumn,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Parse UUID
		parsed, err := uuid.FromString(uuidStr)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID string: %w", err)
		}
		media.UUID = &parsed

		// Set JSON fields
		media.Manipulations = json.RawMessage(manipulations)
		media.CustomProperties = json.RawMessage(customProperties)
		media.GeneratedConversions = json.RawMessage(generatedConversions)
		media.ResponsiveImages = json.RawMessage(responsiveImages)

		// Handle nullable order column
		if orderColumn.Valid {
			orderColumnInt := int(orderColumn.Int32)
			media.OrderColumn = &orderColumnInt
		}

		media.CreatedAt = createdAt
		media.UpdatedAt = updatedAt

		mediaList = append(mediaList, &media)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return mediaList, nil
}

// Save creates or updates a media record
func (r *SQLMediaRepository) Save(ctx context.Context, media *models.Media) error {
	if media.ID == 0 {
		// Insert new record
		query := `
			INSERT INTO media (
				model_type, model_id, uuid, collection_name, name, file_name, 
				mime_type, disk, conversions_disk, size, manipulations, 
				custom_properties, generated_conversions, responsive_images, 
				order_column, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`

		var orderColumnValue interface{} = nil
		if media.OrderColumn != nil {
			orderColumnValue = *media.OrderColumn
		}

		var id uint64
		err := r.db.QueryRowContext(
			ctx,
			query,
			media.ModelType,
			media.ModelID,
			media.UUID.String(),
			media.CollectionName,
			media.Name,
			media.FileName,
			media.MimeType,
			media.Disk,
			media.ConversionsDisk,
			media.Size,
			media.Manipulations,
			media.CustomProperties,
			media.GeneratedConversions,
			media.ResponsiveImages,
			orderColumnValue,
			media.CreatedAt,
			media.UpdatedAt,
		).Scan(&id)

		if err != nil {
			return fmt.Errorf("failed to create media record: %w", err)
		}

		media.ID = id
	} else {
		// Update existing record
		query := `
			UPDATE media 
			SET model_type = ?, model_id = ?, uuid = ?, collection_name = ?, 
				name = ?, file_name = ?, mime_type = ?, disk = ?, 
				conversions_disk = ?, size = ?, manipulations = ?, 
				custom_properties = ?, generated_conversions = ?, 
				responsive_images = ?, order_column = ?, updated_at = ?
			WHERE id = ?
		`

		var orderColumnValue interface{} = nil
		if media.OrderColumn != nil {
			orderColumnValue = *media.OrderColumn
		}

		_, err := r.db.ExecContext(
			ctx,
			query,
			media.ModelType,
			media.ModelID,
			media.UUID.String(),
			media.CollectionName,
			media.Name,
			media.FileName,
			media.MimeType,
			media.Disk,
			media.ConversionsDisk,
			media.Size,
			media.Manipulations,
			media.CustomProperties,
			media.GeneratedConversions,
			media.ResponsiveImages,
			orderColumnValue,
			time.Now(),
			media.ID,
		)

		if err != nil {
			return fmt.Errorf("failed to update media record: %w", err)
		}
	}

	return nil
}

// FindByID retrieves a media record by ID
func (r *SQLMediaRepository) FindByID(ctx context.Context, id uint64) (*models.Media, error) {
	query := `
		SELECT id, model_type, model_id, uuid, collection_name, name, 
		       file_name, mime_type, disk, conversions_disk, size, 
		       manipulations, custom_properties, generated_conversions, 
		       responsive_images, order_column, created_at, updated_at
		FROM media
		WHERE id = ?
	`

	row := r.db.QueryRowContext(ctx, query, id)

	media, err := scanMedia(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find media by ID: %w", err)
	}

	return media, nil
}

// Delete removes a media record
func (r *SQLMediaRepository) Delete(ctx context.Context, media *models.Media) error {
	query := `DELETE FROM media WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, media.ID)
	if err != nil {
		return fmt.Errorf("failed to delete media: %w", err)
	}

	return nil
}

// FindByModelTypeAndID retrieves media records for a specific model
func (r *SQLMediaRepository) FindByModelTypeAndID(ctx context.Context, modelType string, modelID uint64) ([]*models.Media, error) {
	query := `
		SELECT id, model_type, model_id, uuid, collection_name, name, 
		       file_name, mime_type, disk, conversions_disk, size, 
		       manipulations, custom_properties, generated_conversions, 
		       responsive_images, order_column, created_at, updated_at
		FROM media
		WHERE model_type = ? AND model_id = ?
	`

	rows, err := r.db.QueryContext(ctx, query, modelType, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to find media by model: %w", err)
	}
	defer rows.Close()

	return scanMediaList(rows)
}

// FindByCollection retrieves media records for a specific collection
func (r *SQLMediaRepository) FindByCollection(ctx context.Context, collection string) ([]*models.Media, error) {
	query := `
		SELECT id, model_type, model_id, uuid, collection_name, name, 
		       file_name, mime_type, disk, conversions_disk, size, 
		       manipulations, custom_properties, generated_conversions, 
		       responsive_images, order_column, created_at, updated_at
		FROM media
		WHERE collection_name = ?
	`

	rows, err := r.db.QueryContext(ctx, query, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to find media by collection: %w", err)
	}
	defer rows.Close()

	return scanMediaList(rows)
}

// FindByModelAndCollection retrieves media records for a specific model and collection
func (r *SQLMediaRepository) FindByModelAndCollection(ctx context.Context, modelType string, modelID uint64, collection string) ([]*models.Media, error) {
	query := `
		SELECT id, model_type, model_id, uuid, collection_name, name, 
		       file_name, mime_type, disk, conversions_disk, size, 
		       manipulations, custom_properties, generated_conversions, 
		       responsive_images, order_column, created_at, updated_at
		FROM media
		WHERE model_type = ? AND model_id = ? AND collection_name = ?
	`

	rows, err := r.db.QueryContext(ctx, query, modelType, modelID, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to find media by model and collection: %w", err)
	}
	defer rows.Close()

	return scanMediaList(rows)
}

// Verify that SQLMediaRepository implements the MediaRepository interface
var _ medialibrary.MediaRepository = (*SQLMediaRepository)(nil)
