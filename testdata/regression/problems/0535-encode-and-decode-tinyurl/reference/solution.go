package main

import (
	"hash/fnv"
	"strings"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Codec names a URL after its hash in base 62, probing on collisions.
type Codec struct {
	byKey map[string]string
	byURL map[string]string
}

func Constructor() Codec {
	return Codec{byKey: map[string]string{}, byURL: map[string]string{}}
}

func (this *Codec) encode(longUrl string) string {
	if key, ok := this.byURL[longUrl]; ok {
		return "http://tinyurl.com/" + key
	}
	h := fnv.New64a()
	h.Write([]byte(longUrl))
	for n := h.Sum64(); ; n++ {
		key := base62(n)
		if _, taken := this.byKey[key]; !taken {
			this.byKey[key], this.byURL[longUrl] = longUrl, key
			return "http://tinyurl.com/" + key
		}
	}
}

func (this *Codec) decode(shortUrl string) string {
	return this.byKey[shortUrl[strings.LastIndexByte(shortUrl, '/')+1:]]
}

func base62(n uint64) string {
	var b []byte
	for len(b) < 6 || n > 0 {
		b = append(b, alphabet[n%62])
		n /= 62
	}
	return string(b)
}
