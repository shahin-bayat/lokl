package config

func cmdStr(s string) StringOrSlice {
	return StringOrSlice{Args: []string{s}, Shell: true}
}

func cmdArgs(args ...string) StringOrSlice {
	return StringOrSlice{Args: args, Shell: false}
}
