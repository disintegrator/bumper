package cmd

type CommandError struct {
	cause error
}

func (e *CommandError) Error() string {
	return "command failed"
}

func (e *CommandError) Unwrap() error {
	return e.cause
}

func Failed(err error) error {
	return &CommandError{cause: err}
}
