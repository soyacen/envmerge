package cli

import (
	"fmt"

	"github.com/urfave/cli/v3"
)

// singleInput requires exactly one positional template argument.
func singleInput(c *cli.Command) (string, error) {
	args := c.Args().Slice()
	if len(args) != 1 {
		return "", fmt.Errorf("Error: %s requires exactly one template argument", c.Name)
	}
	return args[0], nil
}
