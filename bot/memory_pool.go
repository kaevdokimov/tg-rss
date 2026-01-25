package bot

import (
	"bytes"
	"sync"
)

// Memory pools для уменьшения нагрузки на GC
var (
	// Pool для буферов чтения HTTP ответов
	bufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	// Pool для срезов байтов
	byteSlicePool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 4096) // 4KB initial capacity
		},
	}

	// Pool для map'ов строк
	stringMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]string)
		},
	}

	// Pool для string builder'ов
	stringBuilderPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
)

// GetBuffer получает буфер из pool'а
func GetBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// PutBuffer возвращает буфер в pool
func PutBuffer(buf *bytes.Buffer) {
	buf.Reset() // Очищаем буфер перед возвратом
	bufferPool.Put(buf)
}

// GetByteSlice получает срез байтов из pool'а
func GetByteSlice() []byte {
	return byteSlicePool.Get().([]byte)
}

// PutByteSlice возвращает срез байтов в pool
func PutByteSlice(slice []byte) {
	if cap(slice) <= 65536 { // Не возвращаем очень большие срезы (макс 64KB)
		byteSlicePool.Put((*[]byte)(&slice)) // Передаем указатель на срез
	}
}

// GetStringMap получает map строк из pool'а
func GetStringMap() map[string]string {
	return stringMapPool.Get().(map[string]string)
}

// PutStringMap возвращает map строк в pool
func PutStringMap(m map[string]string) {
	// Очищаем map перед возвратом
	for k := range m {
		delete(m, k)
	}
	stringMapPool.Put(m)
}

// GetStringBuilder получает string builder из pool'а
func GetStringBuilder() *bytes.Buffer {
	return stringBuilderPool.Get().(*bytes.Buffer)
}

// PutStringBuilder возвращает string builder в pool
func PutStringBuilder(sb *bytes.Buffer) {
	sb.Reset() // Очищаем builder перед возвратом
	stringBuilderPool.Put(sb)
}
