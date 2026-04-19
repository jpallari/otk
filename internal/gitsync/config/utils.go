package config

func overrideStr(target *string, source string) {
	if source != "" {
		*target = source
	}
}

func overrideBool(target *bool, source bool) {
	if source {
		*target = source
	}
}
