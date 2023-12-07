package components

type Argument struct {
	Name     string
	Optional bool
	// This field will be used for help usage, creating one usage with the argument and one with its (optional) flag replacement.
	ReplaceWithFlag string
	Description     string
}

type EnvVar struct {
	Name        string
	Default     string
	Description string
}

type ActionFunc func(c *Context) error

type Context struct {
	Arguments        []string
	CommandName      string
	stringFlags      map[string]string
	boolFlags        map[string]bool
	PrintCommandHelp func(commandName string) error
}

func (c *Context) GetStringFlagValue(flagName string) string {
	return c.stringFlags[flagName]
}

func (c *Context) GetBoolFlagValue(flagName string) bool {
	return c.boolFlags[flagName]
}

type Flag interface {
	GetName() string
	IsMandatory() bool
	GetDescription() string
}

type StringFlag struct {
	Name        string
	Description string
	Mandatory   bool
	// A flag with default value cannot be mandatory.
	DefaultValue string
	// Optional. If provided, this field will be used for help usage. --<Name>=<ValueAlias> else: --<Name>=<value>
	ValueAlias string
}

func (f StringFlag) GetName() string {
	return f.Name
}

func (f StringFlag) GetDescription() string {
	return f.Description
}

func (f StringFlag) GetDefault() string {
	return f.DefaultValue
}

func (f StringFlag) IsMandatory() bool {
	return f.Mandatory
}

type BoolFlag struct {
	Name         string
	Description  string
	DefaultValue bool
}

func (f BoolFlag) GetName() string {
	return f.Name
}

func (f BoolFlag) GetDescription() string {
	return f.Description
}

func (f BoolFlag) GetDefault() bool {
	return f.DefaultValue
}

func (f BoolFlag) IsMandatory() bool {
	return false
}
