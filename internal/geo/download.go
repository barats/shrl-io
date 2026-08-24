package geo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	editionID   = "GeoLite2-City"
	downloadURL = "https://download.maxmind.com/app/geoip_download"
	userAgent   = "shrl-io (self-hosted url shortener)"
)

// UpdateInterval is how often the worker refreshes the GeoLite2 database
// (MaxMind ships weekly updates).
const UpdateInterval = 7 * 24 * time.Hour

// Ensure downloads the GeoLite2-City database to path if it does not already
// exist. Without a license key it is a no-op (the caller should treat geo as
// disabled).
func Ensure(ctx context.Context, path, licenseKey string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if licenseKey == "" {
		return nil
	}
	return download(ctx, path, licenseKey)
}

// Update refreshes the database at path.
func Update(ctx context.Context, path, licenseKey string) error {
	if licenseKey == "" {
		return nil
	}
	return download(ctx, path, licenseKey)
}

func download(ctx context.Context, path, licenseKey string) error {
	url := fmt.Sprintf("%s?edition_id=%s&license_key=%s&suffix=tar.gz", downloadURL, editionID, licenseKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("maxmind download: %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".geolite-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeReg && strings.HasSuffix(hdr.Name, editionID+".mmdb") {
			if _, err := io.Copy(tmp, tr); err != nil {
				return err
			}
			found = true
			break
		}
	}
	if !found {
		return errors.New("maxmind download: mmdb not found in archive")
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
