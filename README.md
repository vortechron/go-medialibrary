# Go Media Library

A Go library for handling media files in web applications, inspired by Spatie's Laravel Media Library.

## Features

- Add media from URLs and store in AWS S3
- Add media from local disk and store in different disks
- Generate image conversions (thumbnails, previews, etc.)
- Generate responsive images
- Associate media with different model types (morphing)
- Customizable storage implementations via interfaces
- GORM integration for database persistence
- Optional media naming with smart defaults (uses filename without extension by default)

## Installation

```bash
go get github.com/vortechron/go-medialibrary
```

## Requirements

- Go 1.16+
- AWS credentials configured
- A PostgreSQL database (if using the GORM repository)

## Quick Start

```go
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

func main() {
  // Create a context
  ctx := context.Background()

  // Setup database connection
  db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=postgres dbname=medialibrary port=5432 sslmode=disable"), &gorm.Config{})
  if err != nil {
    log.Fatalf("Failed to connect to database: %v", err)
  }

  // Create the media repository
  repo := repository.NewGormMediaRepository(db)

  // Run migrations
  if err := repo.AutoMigrate(); err != nil {
    log.Fatalf("Failed to run migrations: %v", err)
  }

  // Setup S3 storage
  s3Config := storage.S3Config{
    Bucket:     os.Getenv("S3_BUCKET"),
    Region:     os.Getenv("S3_REGION"),
    BaseURL:    os.Getenv("S3_BASE_URL"),
    PublicURLs: true,
  }

  s3Storage, err := storage.NewS3Storage(ctx, s3Config)
  if err != nil {
    log.Fatalf("Failed to create S3 storage: %v", err)
  }

  // Setup Local storage
  localConfig := storage.LocalConfig{
    BasePath: "/path/to/local/storage",
    BaseURL:  "http://localhost:8080/media",
  }

  localStorage, err := storage.NewLocalStorage(localConfig)
  if err != nil {
    log.Fatalf("Failed to create local storage: %v", err)
  }

  // Create a disk manager to manage different storages
  diskManager := storage.NewDiskManager()
  diskManager.AddDisk("s3", s3Storage)
  diskManager.AddDisk("local", localStorage)

  // Create the image transformer
  transformer := conversion.NewImagingTransformer()
  
  // Register default conversions
  transformer.DefaultConversions()
  
  // Register default responsive image conversions
  transformer.DefaultResponsiveConversions()
  
  // Create the media library
  mediaLib := medialibrary.NewDefaultMediaLibrary(
    diskManager,
    transformer,
    repo,
    medialibrary.WithDefaultDisk("s3"),
    medialibrary.WithConversionsDisk("s3"),
    medialibrary.WithAutoGenerateConversions(true),
    medialibrary.WithPerformConversions([]string{"thumbnail", "preview"}),
    medialibrary.WithGenerateResponsiveImages([]string{"responsive"}),
    medialibrary.WithCustomProperties(map[string]interface{}{
      "default": "value",
    }),
  )

  // Add a media from URL and associate it with a model (e.g., a Post with ID 1)
  media, err := mediaLib.AddMediaFromURL(
    ctx,
    "https://example.com/image.jpg",
    "gallery",
    medialibrary.WithName("Example Image"),
    medialibrary.WithModel("posts", 1),
    medialibrary.WithCustomProperties(map[string]interface{}{
      "alt": "Example image",
      "caption": "This is an example image",
    }),
  )
  if err != nil {
    log.Fatalf("Failed to add media from URL: %v", err)
  }

  // Get media URL
  fmt.Printf("Media URL: %s\n", mediaLib.GetURLForMedia(media))
  
  // Get thumbnail URL
  fmt.Printf("Thumbnail URL: %s\n", mediaLib.GetURLForMediaConversion(media, "thumbnail"))
  
  // Get all media for a specific model
  postMedia, err := mediaLib.GetMediaForModel(ctx, "posts", 1)
  if err != nil {
    log.Fatalf("Failed to get media for post: %v", err)
  }
  
  fmt.Printf("Media for Post ID 1: %d items\n", len(postMedia))

  // Method 2: Using AddMediaFromURLToModel
  media, err = mediaLib.AddMediaFromURLToModel(
    ctx,
    "https://example.com/image.jpg",
    "posts", // Model type
    123,     // Model ID
    "gallery",
    medialibrary.WithName("Image Name"), // Optional: provide a custom name
  )
  if err != nil {
    log.Fatalf("Failed to add media from URL to model: %v", err)
  }
}
```

## Configuration

### Storage Configuration

#### S3 Storage

```go
s3Config := storage.S3Config{
  Bucket:     "your-bucket-name",
  Region:     "us-west-2",
  BaseURL:    "https://your-cdn-url.com", // Optional
  PublicURLs: true, // Whether to generate public URLs
}

s3Storage, err := storage.NewS3Storage(ctx, s3Config)
```

#### Local Storage

```go
localConfig := storage.LocalConfig{
  BasePath: "/path/to/local/storage",
  BaseURL:  "http://localhost:8080/media",
}

localStorage, err := storage.NewLocalStorage(localConfig)
```

### Disk Management

You can use multiple storage implementations as "disks" and switch between them:

```go
// Create a disk manager
diskManager := storage.NewDiskManager()

// Add disks
diskManager.AddDisk("s3", s3Storage)
diskManager.AddDisk("local", localStorage)

// Use the disk manager when creating the media library
mediaLib := medialibrary.NewDefaultMediaLibrary(
  diskManager,
  transformer,
  repo,
  medialibrary.WithDefaultDisk("s3"), // Default disk for storing media
  medialibrary.WithConversionsDisk("local"), // Disk for storing conversions
)

// You can also specify the disk when adding media
media, err := mediaLib.AddMediaFromURL(
  ctx,
  "https://example.com/image.jpg",
  "gallery",
  medialibrary.WithName("Example Image"),
  medialibrary.WithModel("posts", 1),
  medialibrary.WithDisk("local"),
)
```

### Media Library Options

```go
// These are all the available options
mediaLib := medialibrary.NewDefaultMediaLibrary(
  diskManager,
  transformer,
  repository,
  medialibrary.WithDefaultDisk("s3"), // Default storage disk
  medialibrary.WithConversionsDisk("s3"), // Storage for conversions
  medialibrary.WithAutoGenerateConversions(true), // Auto-generate conversions on upload
  medialibrary.WithPerformConversions([]string{"thumbnail", "preview"}), // Which conversions to perform
  medialibrary.WithGenerateResponsiveImages([]string{"responsive"}), // Which responsive image sets to generate
  medialibrary.WithCustomProperties(map[string]interface{}{ // Custom properties to add to all media
    "default": "value",
  }),
)
```

### Media Naming

By default, media files will be named using the source filename without its extension. You can override this with the `WithName` option:

```go
// Using the default name (filename without extension)
media, err := mediaLib.AddMediaFromURL(
  ctx,
  "https://example.com/vacation-photo.jpg", // Will use "vacation-photo" as the name
  "gallery",
)

// Specifying a custom name
media, err := mediaLib.AddMediaFromURL(
  ctx,
  "https://example.com/image123.jpg",
  "gallery",
  medialibrary.WithName("Summer Vacation 2023"), 
)
```

## Working with Disks

The library supports the concept of "disks" similar to Laravel, but with a more explicit Go approach:

```go
// Adding media from a local disk file to a remote disk
media, err := mediaLib.AddMediaFromDisk(
  ctx,
  "/path/to/local/image.jpg", // Local file path
  "gallery",
  medialibrary.WithDisk("s3"), // Target disk where to store the media
)

// Adding media from one disk to another
media, err := mediaLib.AddMediaFromDiskToDisk(
  ctx,
  "local", // Source disk
  "path/relative/to/disk/image.jpg", // Path relative to source disk
  "s3", // Target disk
  "gallery",
  medialibrary.WithName("Custom Image Name"),
)

// Copying media from one disk to another
copiedMedia, err := mediaLib.CopyMediaToDisk(
  ctx,
  existingMedia, // Existing media object
  "backup", // Target disk
)

// Moving media from one disk to another
movedMedia, err := mediaLib.MoveMediaToDisk(
  ctx,
  existingMedia, // Existing media object
  "archive", // Target disk
)
```

## Working with Models (Morphing)

You can associate media with different model types, making it easy to organize media by its relationship to your domain models:

```go
// Method 1: Using AddMediaFromURL with WithModel option
media, err := mediaLib.AddMediaFromURL(
  ctx,
  "https://example.com/image.jpg",
  "gallery",
  medialibrary.WithModel("posts", 123), // Associate with Post ID 123
)

// Method 2: Using AddMediaFromURLToModel
media, err := mediaLib.AddMediaFromURLToModel(
  ctx,
  "https://example.com/image.jpg",
  "posts", // Model type
  123,     // Model ID
  "gallery",
  medialibrary.WithName("Image Name"), // Optional: provide a custom name
)

// Retrieving media for a model
postMedia, err := mediaLib.GetMediaForModel(ctx, "posts", 123)

// Retrieving media for a model and specific collection
galleryMedia, err := mediaLib.GetMediaForModelAndCollection(ctx, "posts", 123, "gallery")
```

## Custom Conversions

You can register custom conversions to transform your images:

```go
transformer := conversion.NewImagingTransformer()

// Register a custom conversion
transformer.RegisterConversion("square", func(img image.Image, opts *conversion.Options) (image.Image, error) {
  // Create a square thumbnail
  return transformer.ResizeImage(img, 300, 300, opts)
})

// Register a custom responsive image conversion
transformer.RegisterResponsiveImageConversion("responsive-card", 
  []int{300, 600, 900, 1200}, // Widths
  conversion.WithQuality(80),
  conversion.WithFit("fill"),
)
```

## Custom Storage Implementations

You can implement your own storage by implementing the `storage.Storage` interface:

```go
type Storage interface {
  Save(ctx context.Context, path string, contents io.Reader, options ...Option) error
  SaveFromURL(ctx context.Context, path string, url string, options ...Option) error
  Get(ctx context.Context, path string) (io.ReadCloser, error)
  Exists(ctx context.Context, path string) (bool, error)
  Delete(ctx context.Context, path string) error
  URL(path string) string
  TemporaryURL(ctx context.Context, path string, expiry int64) (string, error)
}
```

## Custom Repository Implementations

You can implement your own repository by implementing the `medialibrary.MediaRepository` interface:

```go
type MediaRepository interface {
  Save(ctx context.Context, media *models.Media) error
  FindByID(ctx context.Context, id uint64) (*models.Media, error)
  Delete(ctx context.Context, media *models.Media) error
}
```

## License

MIT 