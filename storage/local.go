package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)


type LocalConfig struct {
	BasePath string 
	BaseURL  string 
}


type LocalStorage struct {
	config LocalConfig
}


func NewLocalStorage(config LocalConfig) (*LocalStorage, error) {

	if config.BasePath == "" {
		return nil, fmt.Errorf("base path is required")
	}


	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}


	if config.BaseURL != "" && !strings.HasSuffix(config.BaseURL, "/") {
		config.BaseURL = config.BaseURL + "/"
	}

	return &LocalStorage{
		config: config,
	}, nil
}


func (s *LocalStorage) Save(ctx context.Context, path string, contents io.Reader, options ...Option) error {
	fullPath := filepath.Join(s.config.BasePath, path)


	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}


	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()


	if _, err := io.Copy(file, contents); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}


func (s *LocalStorage) SaveFromURL(ctx context.Context, path string, urlStr string, options ...Option) error {

	_, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}


	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: status code %d", resp.StatusCode)
	}


	return s.Save(ctx, path, resp.Body, options...)
}


func (s *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.config.BasePath, path)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %w", err)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}


func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(s.config.BasePath, path)

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return true, nil
}


func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(s.config.BasePath, path)

	err := os.Remove(fullPath)
	if err != nil {
		if os.IsNotExist(err) {

			return nil
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}


func (s *LocalStorage) URL(path string) string {
	if s.config.BaseURL == "" {
		return "/" + path
	}

	return s.config.BaseURL + path
}


func (s *LocalStorage) TemporaryURL(ctx context.Context, path string, expiry int64) (string, error) {

	return s.URL(path), nil
}
