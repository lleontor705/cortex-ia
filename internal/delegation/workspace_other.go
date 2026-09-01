//go:build !windows

package delegation

func platformCanonicalPath(value string) string { return value }
