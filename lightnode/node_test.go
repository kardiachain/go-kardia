// Package lightnode
package lightnode

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func TestNewLightNode(t *testing.T) {
	type input struct {
		cfg Config
	}
	type output struct {
		node Node
		err  error
	}

	cases := []struct {
		name     string
		args     input
		expected output
	}{
		{
			name: "success",
			args: input{
				cfg: Config{
					Name:    "LightNode",
					DataDir: "",
				},
			},
			expected: output{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node, err := New(c.args.cfg)
			assert.Equal(t, err, c.expected.err, "err should be equal")
			fmt.Println("Node info", node)
			//assert.Equal(t, *node, c.expected.node, "node should be equal")
		})
	}
}
