package flags

func init() {
	registerDefaults(map[string]interface{}{
		"test-default-flag-not-for-real-use": "test-value",
	})
}
