package models

import (
	"encoding/json"
	"time"

	"github.com/gofrs/uuid"
)


type Media struct {
	ID                   uint64          `json:"id" gorm:"primaryKey"`
	ModelType            string          `json:"model_type" gorm:"index:idx_model"`
	ModelID              uint64          `json:"model_id" gorm:"index:idx_model"`
	UUID                 *uuid.UUID      `json:"uuid" gorm:"type:varchar(36);unique"`
	CollectionName       string          `json:"collection_name"`
	Name                 string          `json:"name"`
	FileName             string          `json:"file_name"`
	MimeType             string          `json:"mime_type"`
	Disk                 string          `json:"disk"`
	ConversionsDisk      string          `json:"conversions_disk"`
	Size                 int64           `json:"size"`
	Manipulations        json.RawMessage `json:"manipulations" gorm:"type:json"`
	CustomProperties     json.RawMessage `json:"custom_properties" gorm:"type:json"`
	GeneratedConversions json.RawMessage `json:"generated_conversions" gorm:"type:json"`
	ResponsiveImages     json.RawMessage `json:"responsive_images" gorm:"type:json"`
	OrderColumn          *int            `json:"order_column" gorm:"index"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}
