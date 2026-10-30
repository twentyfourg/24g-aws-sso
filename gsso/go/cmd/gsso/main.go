// Command gsso is a 24G AWS SSO profile helper. See gsso/go/README.md.
package main

import (
	"os"

	"github.com/twentyfourg/24g-aws-sso/gsso/go/internal/gsso"
)

func main() {
	os.Exit(gsso.Run(os.Args[1:]))
}
