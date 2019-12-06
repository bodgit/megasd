package megasd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func containsCue(dir string) (bool, error) {
	d, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer d.Close()

	info, err := d.Stat()
	if err != nil {
		return false, err
	}

	if !info.IsDir() {
		return false, errors.New("not a directory")
	}

	files, err := d.Readdirnames(0)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if filepath.Ext(file) == ".cue" {
			return true, nil
		}
	}

	return false, nil
}

func findDirectories(ctx context.Context, base string) (<-chan string, <-chan error, error) {
	out := make(chan string)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		errc <- filepath.Walk(base, func(dir string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Ignore any hidden files or directories, otherwise we end up fighting with things like Spotlight, etc.
			if info.Name()[0] == '.' {
				if info.Mode().IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Ignore anything that isn't a directory
			if !info.Mode().IsDir() {
				return nil
			}

			select {
			case out <- dir:
			case <-ctx.Done():
				return errors.New("walk cancelled")
			}

			return nil
		})
	}()
	return out, errc, nil
}

func directoryWorker(ctx context.Context, in <-chan string) (<-chan error, error) {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		for dir := range in {
			// TODO Create DB

			if err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Ignore any hidden files or directories, otherwise we end up fighting with things like Spotlight, etc.
				if info.Name()[0] == '.' {
					if info.Mode().IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				// Ignore anything that isn't a normal file
				if !info.Mode().IsRegular() {
					return nil
				}

				// Ignore any file greater than 16 MB
				if info.Size() > 16<<(10*2) {
					return nil
				}

				switch filepath.Ext(file) {
				case ".bin":
					// For any .bin file, if there is a .cue file in the same directory, assume it's a CD track rather than a ROM image
					hasCue, err := containsCue(filepath.Dir(file))
					if err != nil {
						return err
					}
					if hasCue {
						return nil
					}
					fallthrough
				case ".32x", ".md", ".sg", ".sms":
					// Check files are in the "top" directory
					if filepath.Dir(file) != dir {
						return nil
					}
					crc, err := crcFile(file)
					if err != nil {
						return err
					}
					fmt.Println(crc, crcFilename(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))), file)
				case ".cue":
					if filepath.Dir(filepath.Dir(file)) != dir {
						return nil
					}
					crc, err := crcCueFile(file)
					if err != nil {
						return err
					}
					fmt.Println(crc, crcFilename(filepath.Base(filepath.Dir(file))), file)
				default:
					return nil
				}

				return nil
			}); err != nil {
				errc <- err
				return
			}

			// TODO Write out DB
		}
	}()
	return errc, nil
}

func waitForPipeline(errs ...<-chan error) error {
	errc := mergeErrors(errs...)
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
}

func mergeErrors(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error, len(cs))
	wg.Add(len(cs))
	for _, c := range cs {
		go func(c <-chan error) {
			for n := range c {
				out <- n
			}
			wg.Done()
		}(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func Checksum(dir string) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	var errcList []<-chan error

	dirs, errc, err := findDirectories(ctx, dir)
	if err != nil {
		return err
	}
	errcList = append(errcList, errc)

	for i := 0; i < 10; i++ {
		errc, err := directoryWorker(ctx, dirs)
		if err != nil {
			return err
		}
		errcList = append(errcList, errc)
	}

	return waitForPipeline(errcList...)
}
