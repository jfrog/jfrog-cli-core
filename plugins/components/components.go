package components

type App struct {
	Name        string
	Description string
	Version     string
	Commands    []Command
}

type Command struct {
	Name        string
	Description string
	Aliases     []string
	Arguments   []Argument
	Flags       []Flag
	EnvVars     []EnvVar
	Action      ActionFunc
}

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

type PluginSignature struct {
	Name  string `json:"name,omitempty"`
	Usage string `json:"usage,omitempty"`
	// Only used internally in the CLI.
	ExecutablePath string `json:"executablePath,omitempty"`
}
