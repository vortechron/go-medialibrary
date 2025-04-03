package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vortechron/go-medialibrary/conversion"
	"github.com/vortechron/go-medialibrary/medialibrary"
	"github.com/vortechron/go-medialibrary/repository"
	"github.com/vortechron/go-medialibrary/storage"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	ModelTypePost = "posts"
)

func main() {

	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=medialibrary port=5432 sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	repo := repository.NewGormMediaRepository(db)

	if err := repo.AutoMigrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Get S3 configuration from environment variables or use defaults for local development
	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		s3Bucket = "test-media-bucket" // Replace with your test bucket name
	}

	s3Region := os.Getenv("S3_REGION")
	if s3Region == "" {
		s3Region = "us-east-1" // Replace with your preferred region
	}

	s3AccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	// For local testing only, never hardcode credentials in production code
	// if s3AccessKey == "" {
	//     s3AccessKey = "your-access-key-for-testing"
	// }

	s3SecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	// For local testing only, never hardcode credentials in production code
	// if s3SecretKey == "" {
	//     s3SecretKey = "your-secret-key-for-testing"
	// }

	s3Config := storage.S3Config{
		Bucket:     s3Bucket,
		Region:     s3Region,
		BaseURL:    os.Getenv("S3_BASE_URL"),
		PublicURLs: true,
		AccessKey:  s3AccessKey,
		SecretKey:  s3SecretKey,
	}

	s3Storage, err := storage.NewS3Storage(ctx, s3Config)
	if err != nil {
		log.Fatalf("Failed to create S3 storage: %v", err)
	}

	transformer := conversion.NewImagingTransformer()

	transformer.DefaultConversions()

	transformer.DefaultResponsiveConversions()

	diskManager := storage.NewDiskManager()
	diskManager.AddDisk("s3", s3Storage)

	mediaLib := medialibrary.NewDefaultMediaLibrary(
		diskManager,
		transformer,
		repo,
		medialibrary.WithAutoGenerateConversions(true),
		medialibrary.WithPerformConversions([]string{"thumbnail", "preview"}),
		medialibrary.WithGenerateResponsiveImages([]string{"responsive"}),
	)

	media1, err := mediaLib.AddMediaFromURL(
		ctx,
		"https://example.com/image1.jpg",
		"gallery",
		medialibrary.WithName("Image One"),
		medialibrary.WithModel(ModelTypePost, 1),
		medialibrary.WithCustomProperties(map[string]interface{}{
			"alt":     "Example image 1",
			"caption": "This is the first example image",
		}),
	)
	if err != nil {
		log.Fatalf("Failed to add media from URL: %v", err)
	}

	media2, err := mediaLib.AddMediaFromURLToModel(
		ctx,
		"https://example.com/image2.jpg",
		ModelTypePost,
		1,
		"gallery",
		medialibrary.WithName("Image Two"),
		medialibrary.WithCustomProperties(map[string]interface{}{
			"alt":     "Example image 2",
			"caption": "This is the second example image",
		}),
	)
	if err != nil {
		log.Fatalf("Failed to add media to model from URL: %v", err)
	}

	fmt.Printf("Media 1 URL: %s\n", mediaLib.GetURLForMedia(media1))
	fmt.Printf("Media 2 URL: %s\n", mediaLib.GetURLForMedia(media2))

	fmt.Printf("Media 1 Thumbnail URL: %s\n", mediaLib.GetURLForMediaConversion(media1, "thumbnail"))
	fmt.Printf("Media 2 Thumbnail URL: %s\n", mediaLib.GetURLForMediaConversion(media2, "thumbnail"))

	postMedia, err := mediaLib.GetMediaForModel(ctx, ModelTypePost, 1)
	if err != nil {
		log.Fatalf("Failed to get media for post: %v", err)
	}

	fmt.Printf("\nMedia for Post ID 1 (%d items):\n", len(postMedia))
	for i, media := range postMedia {
		fmt.Printf("%d. %s - %s\n", i+1, media.Name, mediaLib.GetURLForMedia(media))
	}

	galleryMedia, err := mediaLib.GetMediaForModelAndCollection(ctx, ModelTypePost, 1, "gallery")
	if err != nil {
		log.Fatalf("Failed to get gallery media for post: %v", err)
	}

	fmt.Printf("\nGallery media for Post ID 1 (%d items):\n", len(galleryMedia))
	for i, media := range galleryMedia {
		fmt.Printf("%d. %s - %s\n", i+1, media.Name, mediaLib.GetURLForMedia(media))
	}
}
