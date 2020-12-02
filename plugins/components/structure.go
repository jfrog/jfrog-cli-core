package components

type App struct {
	Name        string
	Description string
	Version     string
	Commands    []Command
}

type Command struct {
	Name            string
	Description     string
	Aliases         []string
	Arguments       []Argument
	Flags           []Flag
	EnvVars         []EnvVar
	Action          ActionFunc
	SkipFlagParsing bool
}

type PluginSignature struct {
	Name  string `json:"name,omitempty"`
	Usage string `json:"usage,omitempty"`
	// Only used internally in the CLI.
	ExecutablePath string `json:"executablePath,omitempty"`
}
