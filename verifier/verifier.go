package verifier

import "context"

type Verifier interface {
	Setup(context.Context) error
	Verify(context.Context) (bool, error)
	Teardown(context.Context) error
}
