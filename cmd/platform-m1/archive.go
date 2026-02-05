package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractArchive extracts an archive file to the destination directory, automatically
// detecting the format based on file extension (.tar.gz, .tgz, .zip, or raw binary).
func (p *manager) extractArchive(archivePath, destDir string) error {
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz"):
		return p.extractTarGz(archivePath, destDir)
	case strings.HasSuffix(archivePath, ".zip"):
		return p.extractZip(archivePath, destDir)
	default:
		// Treat as raw binary
		destBinDir := filepath.Join(destDir, "Simtezilo", "bin")

		err := os.MkdirAll(destBinDir, 0o755)
		if err != nil {
			return err
		}

		return p.copyFile(archivePath, filepath.Join(destBinDir, updateBinaryName))
	}
}

// extractTarGz extracts a gzip-compressed tar archive to the destination directory,
// preserving file permissions for executable files.
func (p *manager) extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		err = p.extractTarEntry(header, tarReader, destDir)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractTarEntry processes a single entry from a tar archive, creating directories
// or extracting files as appropriate with path traversal protection.
func (p *manager) extractTarEntry(header *tar.Header, tarReader *tar.Reader, destDir string) error {
	target := filepath.Join(destDir, header.Name) //nolint:gosec // controlled input

	if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
		return fmt.Errorf("invalid path in archive: %s", header.Name)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		err := os.MkdirAll(target, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	case tar.TypeReg:
		err := p.extractTarFile(header, tarReader, target)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractTarFile extracts a regular file from a tar archive to the target path,
// creating parent directories as needed and preserving executable permissions.
func (p *manager) extractTarFile(header *tar.Header, tarReader *tar.Reader, target string) error {
	err := os.MkdirAll(filepath.Dir(target), 0o755)
	if err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	outFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	_, err = io.Copy(outFile, tarReader)
	if err != nil {
		outFile.Close()

		return fmt.Errorf("failed to write file: %w", err)
	}

	outFile.Close()

	if header.Mode&0o111 != 0 {
		err = os.Chmod(target, 0o755)
		if err != nil {
			p.log.Warn().Err(err).Str("file", target).Msg("Failed to set executable permissions")
		}
	}

	return nil
}

// extractZip extracts a zip archive to the destination directory, preserving
// file permissions for executable files.
func (p *manager) extractZip(archivePath, destDir string) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer zipReader.Close()

	for _, zipFile := range zipReader.File {
		err = p.extractZipEntry(zipFile, destDir)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractZipEntry processes a single entry from a zip archive, creating directories
// or extracting files as appropriate with path traversal protection.
func (p *manager) extractZipEntry(zipFile *zip.File, destDir string) error {
	target := filepath.Join(destDir, zipFile.Name) //nolint:gosec // controlled input

	if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
		return fmt.Errorf("invalid path in archive: %s", zipFile.Name)
	}

	if zipFile.FileInfo().IsDir() {
		err := os.MkdirAll(target, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		return nil
	}

	return p.extractZipFile(zipFile, target)
}

// extractZipFile extracts a regular file from a zip archive to the target path,
// creating parent directories as needed and preserving executable permissions.
func (p *manager) extractZipFile(zipFile *zip.File, target string) error {
	err := os.MkdirAll(filepath.Dir(target), 0o755)
	if err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	fileReader, err := zipFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in zip: %w", err)
	}
	defer fileReader.Close()

	outFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, fileReader) //nolint:gosec // controlled input
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if zipFile.Mode()&0o111 != 0 {
		err = os.Chmod(target, 0o755)
		if err != nil {
			p.log.Warn().Err(err).Str("file", target).Msg("Failed to set executable permissions")
		}
	}

	return nil
}

// copyFile copies a file from src to dst, creating the destination file if it
// doesn't exist or truncating it if it does. It removes the destination first
// to handle the case where the destination is a running executable (which can't
// be overwritten directly but can be unlinked).
func (p *manager) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	// Remove destination first - this allows replacing running executables
	// (the old file stays in memory until the process exits)
	_ = os.Remove(dst)

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	return nil
}

// createRollbackArchiveFromStaging creates a tgz archive from the contents of the
// staging directory. This is called after a successful update to preserve the
// original files for potential rollback.
func (p *manager) createRollbackArchiveFromStaging(stagingDir string) error {
	p.log.Info().
		Str("archive", p.rollbackArchive()).
		Str("staging", stagingDir).
		Msg("Creating rollback archive from staging")

	// Check if staging directory exists and has files
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return fmt.Errorf("failed to read staging directory: %w", err)
	}

	if len(entries) == 0 {
		p.log.Warn().Msg("Staging directory is empty, no rollback archive created")

		return nil
	}

	// Create archive file
	archiveFile, err := os.Create(p.rollbackArchive())
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer archiveFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Walk the staging directory and add all files to the archive
	fileCount := 0

	walkErr := filepath.Walk(stagingDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Skip the root staging directory itself
		if path == stagingDir {
			return nil
		}

		// Skip directories (we'll create them implicitly via file paths)
		if info.IsDir() {
			return nil
		}

		// Calculate relative path for archive entry
		relPath, relErr := filepath.Rel(stagingDir, path)
		if relErr != nil {
			p.log.Warn().Err(relErr).Str("path", path).Msg("Failed to get relative path")

			return nil
		}

		// Add file to archive
		addErr := p.addFileToArchive(tarWriter, path, relPath)
		if addErr != nil {
			p.log.Warn().
				Err(addErr).
				Str("file", path).
				Msg("Failed to add file to rollback archive")
			// Continue adding other files even if one fails
		} else {
			fileCount++
		}

		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("failed to walk staging directory: %w", walkErr)
	}

	p.log.Info().Int("fileCount", fileCount).Msg("Rollback archive created from staging")

	return nil
}

// addFileToArchive adds a single file to a tar archive with the specified path.
func (p *manager) addFileToArchive(tarWriter *tar.Writer, sourcePath, archivePath string) error {
	// Check if file exists
	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			p.log.Debug().Str("file", sourcePath).Msg("File does not exist, skipping")

			return nil
		}

		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Skip directories
	if fileInfo.IsDir() {
		return nil
	}

	// Open source file
	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create tar header
	header := &tar.Header{
		Name:    archivePath,
		Size:    fileInfo.Size(),
		Mode:    int64(fileInfo.Mode()),
		ModTime: fileInfo.ModTime(),
	}

	// Write header
	err = tarWriter.WriteHeader(header)
	if err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Write file contents
	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return fmt.Errorf("failed to write file to archive: %w", err)
	}

	p.log.Debug().Str("file", archivePath).Msg("Added file to rollback archive")

	return nil
}
