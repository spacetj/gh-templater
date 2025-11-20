package apply

import (
	"fmt"
	"io"
	"os"
)

type stepRunner struct {
	dryRun bool
	writer io.Writer
}

func newStepRunner(dryRun bool, writer io.Writer) stepRunner {
	if writer == nil {
		writer = os.Stdout
	}
	return stepRunner{dryRun: dryRun, writer: writer}
}

func (r stepRunner) Run(description string, action func() error) error {
	if r.dryRun {
		if description == "" {
			description = "(unspecified action)"
		}
		_, err := fmt.Fprintf(r.writer, "[dry-run] %s\n", description)
		return err
	}
	return action()
}

func (r stepRunner) IsDryRun() bool {
	return r.dryRun
}
