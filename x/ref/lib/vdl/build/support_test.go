package build

// SetFilePathSeparator is for use from within tests to fake out working
// on windows style filesystems.
func SetFilePathSeparator(sep string) {
	filePathSeparator = sep
}
