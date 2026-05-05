package extendible

import (
	"fmt"
	"os"
	"sort"
	"syscall"
	"unsafe"
)

func mmapFileRead(path string) (*os.File, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if info.Size() <= 0 {
		_ = f.Close()
		return nil, nil, fmt.Errorf("cannot mmap empty file: %s", path)
	}

	data, err := syscall.Mmap(int(f.Fd()), 0, int(info.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	return f, data, nil
}

func mmapFileWrite(path string, size int) (*os.File, []byte, error) {
	if size <= 0 {
		return nil, nil, fmt.Errorf("invalid mmap size: %d", size)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, nil, err
	}

	if err := f.Truncate(int64(size)); err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	return f, data, nil
}

func msync(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_MSYNC,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(syscall.MS_SYNC),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func closeMappedFile(f *os.File, data []byte, sync bool) error {
	var firstErr error

	if sync {
		if err := msync(data); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if len(data) > 0 {
		if err := syscall.Munmap(data); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if f != nil {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

type mappedRegion struct {
	path  string
	file  *os.File
	data  []byte
	dirty bool
}

type mmapCache struct {
	regions map[string]*mappedRegion
}

func newMmapCache() *mmapCache {
	return &mmapCache{regions: make(map[string]*mappedRegion)}
}

func (c *mmapCache) ensureWritableRegion(path string, size int) (*mappedRegion, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid mmap size: %d", size)
	}

	if region, ok := c.regions[path]; ok {
		if len(region.data) == size {
			return region, nil
		}
		if err := closeMappedFile(region.file, region.data, false); err != nil {
			return nil, err
		}
		delete(c.regions, path)
	}

	f, data, err := mmapFileWrite(path, size)
	if err != nil {
		return nil, err
	}

	region := &mappedRegion{path: path, file: f, data: data}
	c.regions[path] = region
	return region, nil
}

func (c *mmapCache) write(path string, size int, fill func([]byte) error) error {
	region, err := c.ensureWritableRegion(path, size)
	if err != nil {
		return err
	}
	if err := fill(region.data[:size]); err != nil {
		return err
	}
	region.dirty = true
	return nil
}

func (c *mmapCache) flushDirty() error {
	paths := make([]string, 0, len(c.regions))
	for path, region := range c.regions {
		if region.dirty {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	for _, path := range paths {
		region := c.regions[path]
		if err := msync(region.data); err != nil {
			return err
		}
		region.dirty = false
	}

	return nil
}

func (c *mmapCache) remove(path string) error {
	if region, ok := c.regions[path]; ok {
		if err := closeMappedFile(region.file, region.data, false); err != nil {
			return err
		}
		delete(c.regions, path)
	}

	if err := osRemove(path); err != nil && !isNotExist(err) {
		return err
	}
	return nil
}

func (c *mmapCache) close() error {
	if err := c.flushDirty(); err != nil {
		return err
	}

	paths := make([]string, 0, len(c.regions))
	for path := range c.regions {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		region := c.regions[path]
		if err := closeMappedFile(region.file, region.data, false); err != nil {
			return err
		}
		delete(c.regions, path)
	}

	return nil
}
