package utils

import (
	"bytes"
	"io"
)

// ReadCloser 可重用的读取器实现io.ReadCloser接口
type ReadCloser struct {
	*bytes.Reader
}

// Close 关闭读取器，实现io.Closer接口
func (r ReadCloser) Close() error {
	return nil
}

// NewReadCloser 创建新的可重用读取器
func NewReadCloser(data []byte) io.ReadCloser {
	return ReadCloser{bytes.NewReader(data)}
} 