package components

type Argument struct {
	Name        string
	Description string
}

type EnvVar struct {
	Name        string
	Default     string
	Description string
}

type ActionFunc func(c *Context) error

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

type Flag interface {
	GetName() string
	GetUsage() string
}

type StringFlag struct {
	Name  string
	Usage string
	// A flag with default value cannot be mandatory.
	DefaultValue string
	Mandatory    bool
}

func (f StringFlag) GetName() string {
	return f.Name
}

func (f StringFlag) GetUsage() string {
	return f.Usage
}

func (f StringFlag) GetDefault() string {
	return f.DefaultValue
}

func (f StringFlag) isMandatory() bool {
	return f.Mandatory
}

type BoolFlag struct {
	Name         string
	Usage        string
	DefaultValue bool
}

func (f BoolFlag) GetName() string {
	return f.Name
}

func (f BoolFlag) GetUsage() string {
	return f.Usage
}

func (f BoolFlag) GetDefault() bool {
	return f.DefaultValue
}
