package perfect

func mix64(v uint64) uint64 {
	v ^= v >> 30
	v *= 0xbf58476d1ce4e5b9
	v ^= v >> 27
	v *= 0x94d049bb133111eb
	v ^= v >> 31
	return v
}

func hashString64(key string) uint64 {
	const (
		offset uint64 = 1469598103934665603
		prime  uint64 = 1099511628211
	)
	h := offset
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= prime
	}
	return mix64(h)
}

func hashWithSeed(key string, seed uint64) uint64 {
	return mix64(hashString64(key) ^ mix64(seed+0x9e3779b97f4a7c15))
}
