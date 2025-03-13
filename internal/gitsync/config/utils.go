package config

func defaultStr(target *string, source string) {
	if *target == "" {
		*target = source
	}
}

func defaultBool(target *bool, source bool) {
	if !*target {
		*target = source
	}
}

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
