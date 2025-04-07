package components

import (
	"fmt"
	"strconv"
	"strings"
)

type Argument struct {
	Name string
	// Is this argument optional? If so, the 'Optional' field should be set to true.
	// This field is used for creating help usages, for instance if argument is:
	// Argument {
	// 		Name: "optional-arg",
	// 		Optional: true,
	// }
	// The help usage that will be created will be:
	//
	// Usage:
	// 	1) cmd-name [cmd options] [optional-arg]
	//
	// Else, if the argument is mandatory ( Argument { Name: "mandatory-arg" } ), the help usage will be:
	//
	// Usage:
	// 	1) cmd-name [cmd options] <mandatory-arg>
	Optional bool
	// Is this argument optional and can be replaced with a flag?
	// If so, the 'Optional' field should be set to true and the 'ReplaceWithFlag' field should be set to the flag name.
	// This field is used for creating help usages, for instance if argument is:
	// Argument {
	// 		Name: "optional-arg",
	// 		Optional: true,
	// 		ReplaceWithFlag: "flag-replacement",
	// }
	// The help usage that will be created will be:
	//
	// Usage:
	// 	1) cmd-name [cmd options] [optional-arg]
	// 	2) cmd-name [cmd options] --flag-replacement=value
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
	ParentContext    *Context
}

func (c *Context) GetStringFlagValue(flagName string) string {
	return c.stringFlags[flagName]
}

func (c *Context) SetStringFlagValue(flagName string, value string) {
	c.stringFlags[flagName] = value
}

func (c *Context) AddStringFlag(key, value string) {
	if c.stringFlags == nil {
		c.stringFlags = make(map[string]string)
	}
	c.stringFlags[key] = value
}

func (c *Context) AddBoolFlag(key string, value bool) {
	if c.boolFlags == nil {
		c.boolFlags = make(map[string]bool)
	}
	c.boolFlags[key] = value
}

func (c *Context) GetIntFlagValue(flagName string) (value int, err error) {
	parsed, err := strconv.ParseInt(c.GetStringFlagValue(flagName), 0, 64)
	if err != nil {
		err = fmt.Errorf("can't parse int flag '%s': %w", flagName, err)
		return
	}
	value = int(parsed)
	return
}

func (c *Context) GetDefaultIntFlagValueIfNotSet(flagName string, defaultValue int) (value int, err error) {
	if c.IsFlagSet(flagName) {
		parsed, err := strconv.ParseInt(c.GetStringFlagValue(flagName), 0, 64)
		if err != nil {
			err = fmt.Errorf("can't parse int flag '%s': %w", flagName, err)
			return value, err
		}
		value = int(parsed)
		return value, err
	}
	return defaultValue, nil
}

func (c *Context) GetBoolFlagValue(flagName string) bool {
	return c.boolFlags[flagName]
}

func (c *Context) GetBoolTFlagValue(flagName string) bool {
	if c.IsFlagSet(flagName) {
		return c.boolFlags[flagName]
	}
	return true
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

func SetMandatoryFalse() StringFlagOption {
	return func(f *StringFlag) {
		f.Mandatory = false
	}
}

func SetMandatoryTrue() StringFlagOption {
	return func(f *StringFlag) {
		f.Mandatory = true
	}
}

func WithBoolDefaultValueFalse() BoolFlagOption {
	return func(f *BoolFlag) {
		f.DefaultValue = false
	}
}

func WithBoolDefaultValueTrue() BoolFlagOption {
	return func(f *BoolFlag) {
		f.DefaultValue = true
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

func (c *Context) WithDefaultIntFlagValue(flagName string, defValue int) (value int, err error) {
	value = defValue
	if c.IsFlagSet(flagName) {
		var parsed int64
		parsed, err = strconv.ParseInt(c.GetStringFlagValue(flagName), 0, 64)
		if err != nil {
			err = fmt.Errorf("can't parse int flag '%s': %w", flagName, err)
			return
		}
		value = int(parsed)
	}
	return
}

func (c *Context) GetNumberOfArgs() int {
	return len(c.Arguments)
}

func (c *Context) GetArgumentAt(index int) string {
	if len(c.Arguments) > index {
		return c.Arguments[index]
	}
	return ""
}

func (c *Context) GetStringsArrFlagValue(flagName string) (resultArray []string) {
	if c.IsFlagSet(flagName) {
		resultArray = append(resultArray, strings.Split(c.GetStringFlagValue(flagName), ";")...)
	}
	return
}

func (c *Context) GetParent() *Context {
	return c.ParentContext
}
