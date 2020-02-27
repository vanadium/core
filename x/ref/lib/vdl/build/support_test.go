package build

// SetFilePathSeparator is for use from within tests to fake out working
// on windows style filesytems.
func SetFilePathSeparator(sep string) {
	filePathSeparator = sep
}
