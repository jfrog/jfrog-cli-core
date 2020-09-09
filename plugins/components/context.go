package components

type Context struct {
	Arguments   []string
	stringFlags map[string]string
	boolFlags   map[string]bool
}

func (c *Context) GetStringFlagValue(flagName string) string {
	return c.stringFlags[flagName]
}

func (c *Context) GetBoolFlagValue(flagName string) bool {
	return c.boolFlags[flagName]
}
