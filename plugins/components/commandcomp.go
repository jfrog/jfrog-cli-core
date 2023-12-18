package components

import "strconv"

type Argument struct {
	Name     string
	Optional bool
	// Optional. When provided, this field is used for creating help usages.
	// Generating a usage with the argument and another usage where the argument is replaced with the specified flag name. For instance:
	// 1) Without FlagReplacement: .... <Name> <Name2>
	// 2) With FlagReplacement: .... --<FlagReplacement>=<Value> <Name2>
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

func (c *Context) IsFlagSet(flagName string) bool {
	if _, exist := c.stringFlags[flagName]; exist {
		return true
	}
	_, exist := c.boolFlags[flagName]
	return exist
}

type Flag interface {
	GetName() string
	IsMandatory() bool
	GetDescription() string
}

type BaseFlag struct {
	Name        string
	Description string
	Hidden      bool
}

func NewFlag(name, description string) BaseFlag {
	return BaseFlag{Name: name, Description: description}
}

func (f BaseFlag) GetName() string {
	return f.Name
}

func (f BaseFlag) GetDescription() string {
	return f.Description
}

func (f BaseFlag) IsMandatory() bool {
	return false
}

type StringFlag struct {
	BaseFlag
	Mandatory bool
	// A flag with default value cannot be mandatory.
	DefaultValue string
	// Optional. If provided, this field will be used for help usage. --<Name>=<HelpValue> else: --<Name>=<value>
	HelpValue string
}

type StringFlagOption func(f *StringFlag)

func NewStringFlag(name, description string, options ...StringFlagOption) StringFlag {
	f := StringFlag{BaseFlag: NewFlag(name, description)}
	for _, option := range options {
		option(&f)
	}
	return f
}

func (f StringFlag) GetDefault() string {
	return f.DefaultValue
}

func (f StringFlag) IsMandatory() bool {
	return f.Mandatory
}

func WithStrDefaultValue(defaultValue string) StringFlagOption {
	return func(f *StringFlag) {
		f.DefaultValue = defaultValue
	}
}

func WithIntDefaultValue(defaultValue int) StringFlagOption {
	return func(f *StringFlag) {
		f.DefaultValue = strconv.Itoa(defaultValue)
	}
}

func SetMandatory() StringFlagOption {
	return func(f *StringFlag) {
		f.Mandatory = true
	}
}

func WithHelpValue(helpValue string) StringFlagOption {
	return func(f *StringFlag) {
		f.HelpValue = helpValue
	}
}

func SetHiddenStrFlag() StringFlagOption {
	return func(f *StringFlag) {
		f.Hidden = true
	}
}

type BoolFlag struct {
	BaseFlag
	DefaultValue bool
}

type BoolFlagOption func(f *BoolFlag)

func (f BoolFlag) GetDefault() bool {
	return f.DefaultValue
}

func NewBoolFlag(name, description string, options ...BoolFlagOption) BoolFlag {
	f := BoolFlag{BaseFlag: NewFlag(name, description)}
	for _, option := range options {
		option(&f)
	}
	return f
}

func WithBoolDefaultValue(defaultValue bool) BoolFlagOption {
	return func(f *BoolFlag) {
		f.DefaultValue = defaultValue
	}
}

func SetHiddenBoolFlag() BoolFlagOption {
	return func(f *BoolFlag) {
		f.Hidden = true
	}
}
