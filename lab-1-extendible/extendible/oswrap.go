package extendible

import "os"

func removeFile(path string) error {
	return os.Remove(path)
}

func fileNotExist(err error) bool {
	return os.IsNotExist(err)
}
