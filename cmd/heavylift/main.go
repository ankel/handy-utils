package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ankel/handy-utils/pkg/log"
	"golang.org/x/sys/unix"
)

type fileInfo struct {
	path    string
	relPath string
	size    int64
	modTime time.Time
}

func main() {
	olderThanStr := flag.String("older-than", "45d", "Only consider files older than some duration, eg 30d. Default is 45d.")
	upper := flag.Int("upper", 90, "Only start moving if src filesystem is more than x% full.")
	lower := flag.Int("lower", 50, "Move files until src filesystem is less than x% full.")
	logLevelFlag := flag.String("log-level", "info", "Set the logging level (debug, info, warn, error)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flag] <src> <dst>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	l := log.NewText(*logLevelFlag)

	// Check for rsync dependency
	if _, err := exec.LookPath("rsync"); err != nil {
		log.Fatal(l, err, "rsync command not found. This utility depends on rsync.")
	}

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	src := flag.Arg(0)
	dst := flag.Arg(1)

	olderThan, err := parseDuration(*olderThanStr)
	if err != nil {
		log.Fatal(l, err, "Invalid duration for --older-than", "value", *olderThanStr)
	}

	// 1. Filesystem capacity check
	var stat unix.Statfs_t
	if err := unix.Statfs(src, &stat); err != nil {
		log.Fatal(l, err, "Failed to get filesystem stats", "path", src)
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	// Percentage calculation similar to df: used / (used + avail)
	percentFull := 0.0
	if used+avail > 0 {
		percentFull = float64(used) / float64(used+avail) * 100
	}

	l.Info("Filesystem usage", "src", src, "percentFull", fmt.Sprintf("%.2f%%", percentFull), "upper", *upper, "lower", *lower)

	if percentFull <= float64(*upper) {
		l.Info("Filesystem is not full enough yet. Terminating.", "percentFull", percentFull, "upper", *upper)
		return
	}

	targetUsed := float64(used+avail) * float64(*lower) / 100.0
	bytesToFree := int64(float64(used) - targetUsed)
	if bytesToFree < 0 {
		bytesToFree = 0
	}

	l.Info("Targeting free space", "bytesToFree", bytesToFree)

	// 2. File collection & filtering
	var files []fileInfo
	threshold := time.Now().Add(-olderThan)

	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			l.Warn("Error walking path", "path", path, "error", err)
			return nil
		}

		if d.IsDir() {
			l.Debug("Ignore directory", "path", path)
			return nil
		}

		info, err := d.Info()
		if err != nil {
			l.Warn("Error getting info", "path", path, "error", err)
			return nil
		}

		// Exclude symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			l.Debug("Ignore symlink", "path", path)
			return nil
		}

		if info.ModTime().Before(threshold) {
			rel, err := filepath.Rel(src, path)
			if err != nil {
				l.Warn("Error getting relative path", "path", path, "error", err)
				return nil
			}
			files = append(files, fileInfo{
				path:    path,
				relPath: rel,
				size:    info.Size(),
				modTime: info.ModTime(),
			})
		}
		return nil
	})

	if err != nil {
		log.Fatal(l, err, "Error walking directory")
	}

	// 3. File selection (Largest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].size > files[j].size
	})

	var toMove []fileInfo
	var totalSize int64
	for _, f := range files {
		toMove = append(toMove, f)
		l.Debug("Will move", "src", f.relPath, "dst", dst)
		totalSize += f.size
		if totalSize >= bytesToFree {
			break
		}
	}

	if len(toMove) == 0 {
		l.Info("No eligible files found to move.")
		return
	}

	l.Info("Selected files to move", "count", len(toMove), "totalSize", totalSize)

	// 4. Transfer via rsync
	tmpFile, err := os.CreateTemp("", "heavylift-files-*.txt")
	if err != nil {
		log.Fatal(l, err, "Failed to create temporary file")
	}
	defer os.Remove(tmpFile.Name())

	for _, f := range toMove {
		if _, err := tmpFile.WriteString(f.relPath + "\n"); err != nil {
			log.Fatal(l, err, "Failed to write to temporary file")
		}
	}
	tmpFile.Close()

	l.Info("Executing rsync", "src", src, "dst", dst)
	// Ensure src and dst have trailing slashes for rsync consistency
	srcDir := src
	if !strings.HasSuffix(srcDir, string(os.PathSeparator)) {
		srcDir += string(os.PathSeparator)
	}
	dstDir := dst
	if !strings.HasSuffix(dstDir, string(os.PathSeparator)) {
		dstDir += string(os.PathSeparator)
	}

	cmd := exec.Command("rsync", "--archive", "--remove-source-files", "--no-implied-dirs", "--files-from="+tmpFile.Name(), srcDir, dstDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		l.Error("rsync failed", "error", err, "output", string(output))
		// Perform folder check anyway in case of partial success
	} else {
		l.Info("rsync completed successfully")
	}

	// 5. Cleanup empty folders
	l.Info("Cleaning up empty directories")
	for _, f := range toMove {
		cleanEmptyDirs(l, src, filepath.Dir(f.path))
	}
}

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func cleanEmptyDirs(l *slog.Logger, root, dir string) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		l.Error("Failed to get absolute path for root", "path", root, "error", err)
		return
	}
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		l.Error("Failed to get absolute path for dir", "path", dir, "error", err)
		return
	}

	for {
		// Stop if we reached the root or went above it
		if dirAbs == rootAbs || !strings.HasPrefix(dirAbs, rootAbs+string(os.PathSeparator)) {
			l.Debug("Reached root or outside of root, stopping cleanup", "dir", dirAbs, "root", rootAbs)
			break
		}

		entries, err := os.ReadDir(dirAbs)
		if err != nil {
			l.Warn("Failed to read directory", "dir", dirAbs, "error", err)
			break
		}

		if len(entries) == 0 {
			l.Info("Removing empty directory", "dir", dirAbs)
			if err := os.Remove(dirAbs); err != nil {
				l.Warn("Failed to remove directory", "dir", dirAbs, "error", err)
				break
			}
			// Move up to parent
			dirAbs = filepath.Dir(dirAbs)
		} else {
			l.Debug("Directory not empty, stopping cleanup", "dir", dirAbs, "count", len(entries))
			break
		}
	}
}
