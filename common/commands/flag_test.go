package commands

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/urfave/cli"
)

// This test demonstrates an existing bug in the urfave/cli library.
// It highlights that both BoolT and Bool flags have their default values set to false.
// Ideally, the BoolT flag should have a default value of true.
func TestBoolVsBoolTFlag(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		shouldUseBoolT bool
		expectValue    bool
	}{
		{"Resolving flag value using Bool (default false)", []string{"cmd"}, false, false},
		{"Resolving flag value using Bool (default false)", []string{"cmd"}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &cli.App{
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "myflag",
						Usage: "Test boolean flag",
					},
				},
				Action: func(c *cli.Context) error {
					if tt.shouldUseBoolT {
						assert.Equal(t, tt.expectValue, c.BoolT("myflag"), "Expected %v, got %v", tt.expectValue, c.BoolT("myflag"))
					} else {
						assert.Equal(t, tt.expectValue, c.Bool("myflag"), "Expected %v, got %v", tt.expectValue, c.Bool("myflag"))
					}
					return nil
				},
			}

			_ = app.Run(tt.args)
		})
	}
}
