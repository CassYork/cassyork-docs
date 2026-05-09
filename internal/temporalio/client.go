package temporalio

import (
	"fmt"

	"go.temporal.io/sdk/client"
)

func NewClient(address, namespace string) (client.Client, error) {
	opts := client.Options{
		HostPort:  address,
		Namespace: namespace,
	}
	c, err := client.Dial(opts)
	if err != nil {
		return nil, fmt.Errorf("temporal dial: %w", err)
	}
	return c, nil
}
